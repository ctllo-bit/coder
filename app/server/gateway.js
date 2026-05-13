const http = require("http");
const httpProxy = require("http-proxy");
const fs = require("fs");//删除旧 socket 文件。
const WebSocket = require('ws');

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

    // 路径规范化
    if (pathname === BASE_PATH) {
        console.log(`[issampro] 重定向: ${pathname} -> ${BASE_PATH}/`);
        res.writeHead(301, { 'Location': BASE_PATH + '/' });
        res.end();
        return;
    }
    console.log(`[coder] 收到请求: ${pathname}`);

    if (pathname.startsWith(BASE_PATH)) {
        req.url = pathname.substring(BASE_PATH.length) || '/';
        console.log(`[coder] 去掉前缀后路径: ${ req.url}`);
    }

    proxy.web(req, res);//转发 HTTP 到code-server.sock
});



// 处理 WebSocket (可选)
const wss = new WebSocket.Server({ noServer: true });
server.on('upgrade', (request, socket, head) => {
    console.log(`[WebSocket] 收到升级请求 | 路径: ${request.url}`);
    wss.handleUpgrade(request, socket, head, (ws) => {
        console.log(`[WebSocket] 连接成功建立`);
        wss.emit('connection', ws, request);
    });
    proxy.ws(request, socket, head);
});

// 监听文件路径而非端口
server.listen(GATEWAY_SOCKET, () => {
    // 赋予权限
    fs.chmodSync(GATEWAY_SOCKET, '0666');
    console.log(`Server listening on ${GATEWAY_SOCKET}`);
});