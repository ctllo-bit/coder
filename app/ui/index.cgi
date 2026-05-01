#!/bin/sh

PORT=$(grep -oE ':[0-9]+' /var/apps/coder/etc/config.yaml | head -n1 | tr -d :)
PORT="${PORT:-8080}"

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
        html, body {
            width: 100%;
            height: 100%;
            margin: 0;
        }

        html {
            box-sizing: border-box;
        }

        *, *::before, *::after {
            box-sizing: inherit;
        }

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
    const port = isInternalIp ? '${PORT}' : window.location.port;
    const airPort=port?(':'+port):'';

    // 构建目标URL
    const targetURL =protocol + "//" + hostname + airPort;

    let iframe = window.frameElement;
    if(iframe){
        iframe.frameBorder = "0";
        iframe.setAttribute("webkitallowfullscreen", "true");
        iframe.setAttribute("mozallowfullscreen", "true");
        iframe.setAttribute('allow', 'clipboard-read; clipboard-write');
        iframe.setAttribute("sandbox","allow-same-origin allow-scripts allow-forms allow-modals allow-popups allow-popups-to-escape-sandbox allow-downloads");
        iframe.src =targetURL;
    }else{
        window.location.href = targetURL;
    }
    </script>
</body>
</html>
EOF
