# 协议边界

- [实时协议 v1](v1.md)：WebSocket `/v1/ws`，用于注册、登录、令牌恢复、角色列表与创建、进入与离开世界。服务端实现在`server/internal/transport/ws`，客户端实现在`client/scripts/net/`。
- HTTP 诊断接口：`GET /healthz`、`GET /version`，不承载账号或游戏数据。

尚未实现：移动和战斗同步（M3/M4）、TLS（正式环境须用`wss://`）、跨 IP 的限流。不得把 Godot 高层 RPC 当作与 Go 兼容的协议。金币一律用整数，物品用唯一 ID，所有结果由服务端裁定。
