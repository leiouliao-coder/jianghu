# Jianghu Go 服务端骨架

要求 Go 1.25 或以上。HTTP 提供诊断接口；配置数据库后另有 WebSocket 接口 `/v1/ws`（注册、登录、角色、进入/离开江湖，见 [`shared/protocol/v1.md`](../shared/protocol/v1.md)）。**尚未实现多人世界、移动、战斗或交易。** 第三方 Go 依赖：`github.com/jackc/pgx/v5`（PostgreSQL）、`github.com/coder/websocket`、`golang.org/x/crypto`（argon2id 密码哈希；固定为支持 Go 1.25 的最后版本 v0.55.0）。

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

## 数据库（可选）

不设置 `JIANGHU_DATABASE_URL` 时服务端不连数据库，只提供诊断接口，`/v1/ws` 返回 404。设置后，启动时加载并校验 `JIANGHU_DATA_DIR`（默认 `../shared/data`），连接数据库并执行内置迁移；任一步失败即以退出码 1 停止。

```sh
docker compose up -d          # 在 server/ 目录，启动本机开发用 PostgreSQL 16（仅监听 127.0.0.1）
JIANGHU_DATABASE_URL='postgres://jianghu:jianghu-dev@127.0.0.1:5432/jianghu?sslmode=disable' go run ./cmd/jianghu-server
```

`compose.yaml` 中的口令只是本机开发占位，不得用于任何可被外部访问的环境。

数据库集成测试需要一个允许建库的 PostgreSQL 账号；每个测试新建独立数据库、结束后删除：

```sh
JIANGHU_TEST_DATABASE_URL='postgres://jianghu:jianghu-dev@127.0.0.1:5432/postgres?sslmode=disable' go test -race ./...
```

未设置该变量时这些测试会跳过并说明原因；CI 设置了 `JIANGHU_REQUIRE_DB=1`，缺数据库即失败，不能静默跳过。

资产规则见 `internal/store`：每次写入带 request_id，重放返回首次结果；物品每行一个堆叠、只在一个容器；钱币是容器内的实物数量，数据库约束禁止为负；容器按 ID 顺序加锁防止并发超额。

## 手机局域网联调

确需其他设备访问时，显式启动：

```sh
JIANGHU_LISTEN_ADDR=0.0.0.0:8080 go run ./cmd/jianghu-server
```

手机访问开发电脑的局域网 IP，不能使用手机自己的 `127.0.0.1`。只在可信局域网并配置防火墙使用；该骨架没有 TLS、认证和限流，**不得直接暴露公网，不要为联调关闭整个防火墙**。iOS / Android 的明文 HTTP 权限或限制仍需客户端后续单独验证。

## 后续模块边界

- `internal/config`：运行参数。
- `internal/transport/httpapi`：诊断传输层。
- `internal/gamedata`：加载并校验 `shared/data`。
- `internal/growth`、`crafting`、`economy`、`ledger`：确定性规则（M0）。
- `internal/store`：PostgreSQL 迁移、幂等请求事务与物品 / 钱币操作（M1），账号凭据与会话（M2）。
- `internal/auth`：argon2id 密码哈希、名字校验、会话令牌（只存哈希）。
- `internal/session`：账号在场租约（同号只有一个角色在世界，断线保留 30 秒）。
- `internal/transport/ws`：WebSocket 网关。

后续增加实时协议、权威地图与战斗；不要把客户端上传的位置、伤害或金币视为可信结果。
