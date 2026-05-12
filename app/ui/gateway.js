const http = require('http');
const fs = require('fs');

const socketPath = '/var/apps/coder/target/gateway.sock';

// 如果文件存在则删除
if (fs.existsSync(socketPath)) {
    fs.unlinkSync(socketPath);
}

const server = http.createServer((req, res) => {
    res.writeHead(200);
    res.end('Hello via Unffdsfdsfdfsdfsddfix Socket');
});

// 监听文件路径而非端口
server.listen(socketPath, () => {
    // 赋予权限
    fs.chmodSync(socketPath, '0666');
    console.log(`Server listening on ${socketPath}`);
});