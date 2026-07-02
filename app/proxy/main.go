package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	codeServerSocket = flag.String("socket", "/var/apps/coder/var/code-server.sock", "upstream code-server unix socket")
	proxySocket      = flag.String("proxy-socket", "/var/apps/coder/target/coder-proxy.sock", "this proxy's own unix socket")
	prefix           = flag.String("prefix", "/app/coder", "URL prefix to strip before forwarding")
)

func main() {
	flag.Parse()
	overrideEnv(flag.CommandLine)

	backend := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: "unix"})
	backend.Director = func(r *http.Request) {
		if strings.HasPrefix(r.URL.Path, *prefix) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, *prefix)
			if !strings.HasPrefix(r.URL.Path, "/") {
				r.URL.Path = "/" + r.URL.Path
			}
		}
		r.URL.Scheme = "http"
		r.URL.Host = "unix"
	}
	backend.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", *codeServerSocket)
		},
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}
	backend.ModifyResponse = func(r *http.Response) error {
		if loc := r.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, *prefix) {
			r.Header.Set("Location", *prefix+loc)
		}
		return nil
	}
	backend.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error: %s %s -> %v", r.Method, r.URL.Path, err)
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such file") {
			http.Error(w, "code-server is not running or the socket does not exist", http.StatusServiceUnavailable)
		} else {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" || strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
			proxyWebSocket(w, r, *codeServerSocket, *prefix)
			return
		}
		backend.ServeHTTP(w, r)
	})

	if err := os.RemoveAll(*proxySocket); err != nil {
		log.Fatalf("failed to remove old proxy socket: %v", err)
	}
	listener, err := net.Listen("unix", *proxySocket)
	if err != nil {
		log.Fatalf("failed to listen on proxy socket %s: %v", *proxySocket, err)
	}
	if err := os.Chmod(*proxySocket, 0666); err != nil {
		log.Printf("warning: failed to set socket permissions: %v", err)
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down proxy...")
		listener.Close()
	}()

	log.Printf("coder proxy listening on %s", *proxySocket)
	log.Printf("upstream: unix://%s  prefix=%s", *codeServerSocket, *prefix)
	if err := http.Serve(listener, handler); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		log.Fatalf("server error: %v", err)
	}
	os.RemoveAll(*proxySocket)
}

func overrideEnv(fs *flag.FlagSet) {
	fs.VisitAll(func(f *flag.Flag) {
		envKey := "CODER_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, ok := os.LookupEnv(envKey); ok {
			f.Value.Set(v)
		}
	})
}

func proxyWebSocket(w http.ResponseWriter, r *http.Request, codeServerSocket, prefix string) {
	path := r.URL.Path
	if strings.HasPrefix(path, prefix) {
		path = strings.TrimPrefix(path, prefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	conn, err := net.DialTimeout("unix", codeServerSocket, 10*time.Second)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot reach code-server socket: %v", err), http.StatusBadGateway)
		return
	}
	defer conn.Close()

	upgradeReq := fmt.Sprintf("GET %s HTTP/1.1\r\n", path)
	upgradeReq += "Host: unix\r\n"
	upgradeReq += "Upgrade: websocket\r\n"
	upgradeReq += "Connection: Upgrade\r\n"
	if secKey := r.Header.Get("Sec-WebSocket-Key"); secKey != "" {
		upgradeReq += fmt.Sprintf("Sec-WebSocket-Key: %s\r\n", secKey)
	}
	if secVer := r.Header.Get("Sec-WebSocket-Version"); secVer != "" {
		upgradeReq += fmt.Sprintf("Sec-WebSocket-Version: %s\r\n", secVer)
	}
	if secProto := r.Header.Get("Sec-WebSocket-Protocol"); secProto != "" {
		upgradeReq += fmt.Sprintf("Sec-WebSocket-Protocol: %s\r\n", secProto)
	}
	if secExt := r.Header.Get("Sec-WebSocket-Extensions"); secExt != "" {
		upgradeReq += fmt.Sprintf("Sec-WebSocket-Extensions: %s\r\n", secExt)
	}
	if cookie := r.Header.Get("Cookie"); cookie != "" {
		upgradeReq += fmt.Sprintf("Cookie: %s\r\n", cookie)
	}
	upgradeReq += "\r\n"

	if _, err := conn.Write([]byte(upgradeReq)); err != nil {
		log.Printf("websocket send upgrade: %v", err)
		return
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		log.Printf("websocket read response: %v", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("websocket hijack: %v", err)
		return
	}
	defer clientConn.Close()

	resp.Write(clientConn)

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(conn, clientConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, conn)
		done <- struct{}{}
	}()
	<-done
}
