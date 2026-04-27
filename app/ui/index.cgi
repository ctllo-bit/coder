#!/bin/sh

# 从配置文件获取的端口
PORT=$(sed -n 's/bind-addr:.*:\([0-9]*\)/\1/p' /var/apps/coder/shares/data/config.yaml)

# 添加 HTTP 响应头
echo "Content-Type: text/html; charset=utf-8"
echo ""

cat <<EOF

<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>VS Code</title>
    <style>
        /* 基础尺寸保证 */
        html, body {
        width: 100%;
        height: 100%;
        margin: 0;
        }

        /* 盒模型统一 */
        html {
        box-sizing: border-box;
        }
        *, *::before, *::after {
        box-sizing: inherit;
        }

        /* 只对根容器禁用滚动 */
        #app, #root, .container {
        width: 100%;
        height: 100%;
        overflow: hidden;
        }
    </style>
</head>
<body>
  <script>
    const host = window.location.hostname;     
    const isInternalIp = /^(?:10|127|172\.(?:1[6-9]|2\d|3[01])|192\.168|169\.254|100\.64)\./.test(host) || host === 'localhost';

    const protocol= window.location.protocol;
    const hostname=isInternalIp ? host : ('coder.'+ host);
    const port = isInternalIp ? '${PORT:-8080}' : window.location.port;
    const airPort=port?(':'+port):'';

    // 构建目标URL
    const targetURL =protocol + "//" + hostname + airPort;

    // 尝试获取当前父级 iframe 元素
    let iframe = window.frameElement;
    if(iframe){
        iframe.frameBorder = "0";
        iframe.setAttribute("webkitallowfullscreen", "true");
        iframe.setAttribute("mozallowfullscreen", "true");
        iframe.setAttribute('allow', 'clipboard-read; clipboard-write');

        //沙盒属性
        iframe.sandbox="allow-same-origin allow-scripts allow-forms allow-modals allow-popups allow-popups-to-escape-sandbox allow-downloads"
        iframe.src =targetURL;

        console.log('✅ 修改iframe成功123！');
    }else{
        //飞牛APP因跨源（cross-origin)获取不了，则直接跳转
        //window.location.href = targetURL;
        window.open(targetURL, '_top');
        window.alert(5 + 6);
    }

    console.log("CGI加载的JS文件成功！");
  </script>
</body>
</html>

EOF