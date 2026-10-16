# Jianghu Go 服务端骨架

要求 Go 1.25 或以上。当前仅提供 HTTP 诊断接口，**没有实现多人世界、战斗、账号、数据库或经济交易**。无第三方 Go 依赖；不需要 `go.sum`。

在仓库的 `server` 目录执行：

```sh
go run ./cmd/jianghu-server
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/version
go test -race ./...
go vet ./...
go build -o /tmp/jianghu-server ./cmd/jianghu-server
```

- `GET /healthz`：`{"service":"jianghu-server","status":"ok"}`，仅说明进程可响应，不代表数据库就绪。
- `GET /version`：`{"service":"jianghu-server","version":"bootstrap"}`。
- 已知路径其他方法返回 405；未知路径返回 404；错误响应为 JSON。
- 默认只监听 `127.0.0.1:8080`。配置来源为进程环境；`configs/.env.example` 不会被自动加载。
- 读取请求头超时 5 秒，读写超时各 10 秒，空闲超时 60 秒；最大请求头 16 KiB。
- SIGINT / SIGTERM 停止接收新连接，并等待现有请求最多 10 秒；再次发终止信号可强制退出。服务只支持普通 HTTP，没有 WebSocket。

## 手机局域网联调

确需其他设备访问时，显式启动：

```sh
JIANGHU_LISTEN_ADDR=0.0.0.0:8080 go run ./cmd/jianghu-server
```

手机访问开发电脑的局域网 IP，不能使用手机自己的 `127.0.0.1`。只在可信局域网并配置防火墙使用；该骨架没有 TLS、认证和限流，**不得直接暴露公网，不要为联调关闭整个防火墙**。iOS / Android 的明文 HTTP 权限或限制仍需客户端后续单独验证。

## 后续模块边界

`internal/config` 加载运行参数，`internal/transport/httpapi` 为诊断传输层。后续再增加权威地图 / 战斗、资产事务与 PostgreSQL 适配层；不要把客户端上传的位置、伤害或金币视为可信结果。客户端与服务端自定义协议尚未实现。
