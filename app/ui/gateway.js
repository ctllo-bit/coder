const http = require("http");
const httpProxy = require("http-proxy");
const fs = require("fs");//删除旧 socket 文件。

const GATEWAY_SOCKET ="/var/apps/coder/target/gateway.sock";
const CODE_SOCKET ="/var/apps/coder/target/code-server.sock";

try {//删除旧 socket
  fs.unlinkSync(GATEWAY_SOCKET);
} catch {}

//创建代理对象
const proxy = httpProxy.createProxyServer({
  target: {
    socketPath: CODE_SOCKET
  },
  ws: true
});


//普通 HTTP 请求处理器,创建 HTTP 服务器。
const server = http.createServer((req, res) => {

  req.url = req.url.replace(/^\/app\/coder(?=\/|$)/, '') || '/';

  proxy.web(req, res);//转发 HTTP 请求
});



server.listen(GATEWAY_SOCKET, () => {
  console.log("gateway listening:", GATEWAY_SOCKET);
});