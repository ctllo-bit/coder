coder-proxy-linux-amd64

GOOS=linux GOARCH=amd64 go build -o coder-proxy-linux-amd64 main.go

GOOS=linux GOARCH=arm64 go build -o coder-proxy-linux-arm64 main.go