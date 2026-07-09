package main

import (
	"context"
	"flag"
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

func proxyWebSocket(w http.ResponseWriter, r *http.Request, codeServerSocket, prefix string) {
	proxy := &httputil.ReverseProxy{
		// Director 负责修改发往后端的请求
		Director: func(req *http.Request) {
			path := req.URL.Path
			if strings.HasPrefix(path, prefix) {
				path = strings.TrimPrefix(path, prefix)
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
			}
			req.URL.Scheme = "http"
			req.URL.Host = "unix"
			req.URL.Path = path

			// ====== 核心修复点 ======
			// 让后端 code-server 看到真实的外部 Host
			req.Host = r.Host
			// 暴力但最有效的做法：直接删掉 Origin 头，绕过 code-server 严格的同源检测
			req.Header.Del("Origin")
		},
		Transport: &http.Transport{
			// Transport 负责底层的网络连接，拦截拨号逻辑，强制连接到指定的 unix socket
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.DialTimeout("unix", codeServerSocket, 10*time.Second)
			},
		},
	}

	// 执行代理，它会自动处理 HTTP 和 WebSocket 升级！
	proxy.ServeHTTP(w, r)
}
