# DCache 打包说明

DCache 是基于一致性哈希的标准库分布式缓存演示服务。

```bash
go test ./...
go vet ./...
go build ./...
go run . -addr 127.0.0.1:7301 -http 127.0.0.1:8080
./build_benzhi_docker.sh dcache linux/amd64
```

TCP 服务默认监听 7301，Web 控制台默认监听 8080。
