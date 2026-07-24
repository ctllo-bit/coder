package main

import (
	"context"
	"errors"
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

	backend := &httputil.ReverseProxy{
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
			// 自动处理 X-Forwarded-* 头
			pr.SetXForwarded()

			// 当 Origin 不为空时：触发了 WebSocket 握手或 API 提交
			if origin := pr.In.Header.Get("Origin"); origin != "" {
				if u, err := url.Parse(origin); err == nil && u.Host != "" {
					u.Host = u.Hostname()
					pr.Out.Header.Set("Origin", u.String())
				}
			}
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", *codeServerSocket)
			},
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second, // 优化对 100-continue 响应的处理
		},
		ModifyResponse: func(r *http.Response) error {
			// 修改重定向 Header 加上 Prefix
			if loc := r.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, *prefix) {
				r.Header.Set("Location", *prefix+loc)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %s %s -> %v", r.Method, r.URL.Path, err)

			// 使用 errors.Is 精确匹配底层系统错误，替代脆弱的字符串匹配
			if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, os.ErrNotExist) {
				http.Error(w, "code-server is not running or the socket does not exist", http.StatusServiceUnavailable)
				return
			}

			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	// 启动前清理残留旧 Socket
	if err := os.RemoveAll(*proxySocket); err != nil {
		log.Fatalf("failed to remove old proxy socket: %v", err)
	}

	listener, err := net.Listen("unix", *proxySocket)
	if err != nil {
		log.Fatalf("failed to listen on proxy socket %s: %v", *proxySocket, err)
	}
	// 利用 defer 确保退出或 panic 时都能安全清理 Socket
	defer os.RemoveAll(*proxySocket)

	if err := os.Chmod(*proxySocket, 0666); err != nil {
		log.Printf("warning: failed to set socket permissions: %v", err)
	}

	server := &http.Server{
		Handler: backend,
		// 设置读取请求头的超时时间，防止 Slowloris 攻击。
		// 注意：千万不要设置 ReadTimeout/WriteTimeout，否则会切断 code-server 的 WebSocket。
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 优雅停机逻辑
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down proxy...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("proxy shutdown error: %v", err)
		}
	}()

	log.Printf("coder proxy listening on %s", *proxySocket)
	log.Printf("upstream: unix://%s  prefix=%s", *codeServerSocket, *prefix)

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}
