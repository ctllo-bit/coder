package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
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

	// 不再使用 httputil.NewSingleHostReverseProxy，直接初始化 ReverseProxy 结构体
	backend := &httputil.ReverseProxy{
		// ====== 使用 Rewrite 替代 Director ======
		Rewrite: func(pr *httputil.ProxyRequest) {
			// 处理路径前缀剥离
			path := pr.In.URL.Path
			if strings.HasPrefix(path, *prefix) {
				path = strings.TrimPrefix(path, *prefix)
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
			}

			// 设置目标 Scheme 和 Host
			pr.Out.URL.Scheme = "http"
			pr.Out.URL.Host = "unix"
			pr.Out.URL.Path = path
			// 将外部真实的 Host 传递给后端
			pr.Out.Host = pr.In.Host
			// 删除 Origin 头，绕过 code-server 严格的同源检测
			pr.Out.Header.Del("Origin")
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", *codeServerSocket)
			},
			MaxIdleConns:    100,
			IdleConnTimeout: 90 * time.Second,
		},
		ModifyResponse: func(r *http.Response) error {
			if loc := r.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, *prefix) {
				r.Header.Set("Location", *prefix+loc)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %s %s -> %v", r.Method, r.URL.Path, err)
			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such file") {
				http.Error(w, "code-server is not running or the socket does not exist", http.StatusServiceUnavailable)
			} else {
				http.Error(w, "Bad Gateway", http.StatusBadGateway)
			}
		},
	}

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

	// 优雅停机逻辑
	server := &http.Server{
		Handler: backend, // 直接将 backend 作为 Server 的 Handler
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down proxy...")

		// 给活动连接 5 秒钟时间完成关闭
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("proxy shutdown error: %v", err)
		}
	}()

	log.Printf("coder proxy listening on %s", *proxySocket)
	log.Printf("upstream: unix://%s  prefix=%s", *codeServerSocket, *prefix)

	// 使用 server.Serve 替代 http.Serve
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	os.RemoveAll(*proxySocket)
}
