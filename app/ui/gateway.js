const http = require("http");
const httpProxy = require("http-proxy");
const fs = require("fs");

const GATEWAY_SOCKET =
  "/var/apps/coder/target/gateway.sock";

const CODE_SOCKET =
  "/var/apps/coder/target/code-server.sock";

try {
  fs.unlinkSync(GATEWAY_SOCKET);
} catch {}

const proxy = httpProxy.createProxyServer({
  target: {
    socketPath: CODE_SOCKET
  },
  ws: true
});

function rewrite(req) {
  req.url = req.url.replace(/^\/app\/coder(?=\/|$)/, '') || '/';

  console.log(req.method, req.url);
}

const server = http.createServer((req, res) => {
  rewrite(req);

  proxy.web(req, res);
});

server.on("upgrade", (req, socket, head) => {
  rewrite(req);

  proxy.ws(req, socket, head);
});

server.listen(GATEWAY_SOCKET, () => {
  console.log("gateway listening:", GATEWAY_SOCKET);
});