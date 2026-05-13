const http = require("http");
const httpProxy = require("http-proxy");
const fs = require("fs");//删除旧 socket 文件。

const BASE_PATH = '/app/coder';
const GATEWAY_SOCKET ="/var/apps/coder/target/gateway.sock";
const CODESERVER_SOCKET ="/var/apps/coder/target/code-server.sock";

// 如果旧socket存在则删除
if (fs.existsSync(GATEWAY_SOCKET)) {
    fs.unlinkSync(GATEWAY_SOCKET);
}

//创建代理对象，把收到的请求转发给 code-server.sock
const proxy = httpProxy.createProxyServer({
  target: {
    socketPath: CODESERVER_SOCKET
  },
  ws: true
});

//创建HTTP Server
const server = http.createServer((req, res) => {
    // 解析 URL
    const parsedUrl = new URL(req.url, 'http://localhost');
    let pathname;
    try {
        pathname = decodeURIComponent(parsedUrl.pathname);
    } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'URL解码失败' }));
        return;
    }

    if (pathname.startsWith(BASE_PATH)) {
        if (pathname === BASE_PATH) {
            res.writeHead(301, { 'Location': BASE_PATH + '/' });//如果访问的是 /app/coder 且没带斜杠，强制重定向到带斜杠的版本
            res.end();
            return;
        }
        req.url = pathname.substring(BASE_PATH.length) || '/';
        console.log(`[issampro] 去掉前缀后路径: ${pathname}`);
    }

    proxy.web(req, res);//转发 HTTP 到code-server.sock
    
});

server.on("upgrade", (req, socket, head) => {
  proxy.ws(req, socket, head);
});

// 监听文件路径而非端口
server.listen(GATEWAY_SOCKET, () => {
    // 赋予权限
    fs.chmodSync(GATEWAY_SOCKET, '0666');
    console.log(`Server listening on ${GATEWAY_SOCKET}`);
});