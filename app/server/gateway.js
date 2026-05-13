const http = require("http");
const httpProxy = require("http-proxy");
const fs = require("fs");

const BASE_PATH = '/app/coder';
const GATEWAY_SOCKET ="/var/apps/coder/target/gateway.sock";
const CODESERVER_SOCKET ="/var/apps/coder/target/code-server.sock";

// 如果旧socket存在则删除
if (fs.existsSync(GATEWAY_SOCKET)) {
    fs.unlinkSync(GATEWAY_SOCKET);
}

//创建代理对象，把收到的请求转发给 code-server.sock
const proxy = httpProxy.createProxyServer({
  target: { socketPath: CODESERVER_SOCKET },
  ws: true
});

// 错误处理，防止代理崩溃
proxy.on('error', (err, req, res) => {
    console.error('[Proxy Error]:', err);
    if (res.writeHead) {
        res.writeHead(500, { 'Content-Type': 'text/plain' });
        res.end('Proxy Error');
    }
});





//创建HTTP Server
const server = http.createServer((req, res) => {
    if (req.url === BASE_PATH) {
        res.writeHead(301, { 'Location': BASE_PATH + '/' });
        return res.end();
    }

    // 2. 只有匹配前缀才转发
    if (req.url.startsWith(BASE_PATH)) {
        // 关键点：重写 url 传给 code-server

        req.url = req.url.substring(BASE_PATH.length) || '/';
        console.log(`[coder] 去掉前缀后路径: ${ req.url}`);
        
        // 转发 HTTP 请求
        proxy.web(req, res);
    } else {
        res.writeHead(404);
        res.end("Not Found");
    }



    // // 解析 URL
    // const parsedUrl = new URL(req.url, 'http://localhost');
    // let pathname;
    // try {
    //     pathname = decodeURIComponent(parsedUrl.pathname);
    // } catch (e) {
    //     res.writeHead(400, { 'Content-Type': 'application/json' });
    //     res.end(JSON.stringify({ error: 'URL解码失败' }));
    //     return;
    // }

    // // 路径规范化
    // if (pathname === BASE_PATH) {
    //     console.log(`[issampro] 重定向: ${pathname} -> ${BASE_PATH}/`);
    //     res.writeHead(301, { 'Location': BASE_PATH + '/' });
    //     res.end();
    //     return;
    // }
    // console.log(`[coder] 收到请求: ${pathname}`);

    // if (pathname.startsWith(BASE_PATH)) {
    //     req.url = pathname.substring(BASE_PATH.length) || '/';
    //     console.log(`[coder] 去掉前缀后路径: ${ req.url}`);
    // }

    // proxy.web(req, res);//转发 HTTP 到code-server.sock
});


server.on('upgrade', (req, socket, head) => {
    console.log(`[WebSocket] 收到升级请求 | 路径: ${req.url}`);
    if (req.url.startsWith(BASE_PATH)) {
        const oldUrl = req.url;
        req.url = req.url.substring(BASE_PATH.length) || '/';

        console.log(`[Proxy Debug] 原始: ${oldUrl} -> 转发至: ${req.url}`);
        proxy.ws(req, socket, head);
    } else {
        socket.destroy();
    }
});


// // 处理 WebSocket (可选)
// const wss = new WebSocket.Server({ noServer: true });
// server.on('upgrade', (request, socket, head) => {
//     
//     wss.handleUpgrade(request, socket, head, (ws) => {
//         console.log(`[WebSocket] 连接成功建立`);
//         wss.emit('connection', ws, request);
//         proxy.ws(request, socket, head);
//     });
    
// });

// 监听文件路径而非端口
server.listen(GATEWAY_SOCKET, () => {
    // 赋予权限
    fs.chmodSync(GATEWAY_SOCKET, '0666');
    console.log(`Server listening on ${GATEWAY_SOCKET}`);
});