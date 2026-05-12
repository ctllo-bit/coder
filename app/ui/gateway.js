const http = require("http");
const httpProxy = require("http-proxy");
const fs = require("fs");//删除旧 socket 文件。

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
    //gateway.sock接收 HTTP 请求,并处理处理 req
    req.url = req.url.replace(/^\/app\/coder(?=\/|$)/, '') || '/';
    proxy.web(req, res);//转发 HTTP 到code-server.sock
});

// 监听文件路径而非端口
server.listen(GATEWAY_SOCKET, () => {
    // 赋予权限
    fs.chmodSync(GATEWAY_SOCKET, '0666');
    console.log(`Server listening on ${GATEWAY_SOCKET}`);
});