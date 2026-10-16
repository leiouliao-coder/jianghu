# Godot 客户端基础工程

使用 Godot 4 标准版与类型化 GDScript，无第三方插件。当前有启动页（`GET /healthz` 连接检查）、账号界面（注册、登录、角色列表与创建）和六区块灰盒世界（服务端寻路移动、其他玩家可见、风险区确认），全部走 WebSocket 协议 v1。**还没有战斗、物品或任务。**

## 打开与运行

在 Godot 项目管理器导入 `client/project.godot`，按 F6 运行主场景或 F5 运行工程。

从仓库根目录也可执行（将 `godot` 替换为本机 Godot 可执行文件路径）：

```sh
godot --editor --path client
godot --path client
```

先按 `server/README.md` 启动 Go 服务，然后点击“检查服务端”。默认访问 `http://127.0.0.1:8080/healthz`，预期 HTTP 200 且 JSON `status` 为 `ok`。地址栏填写服务器基础地址，不含 `/healthz`。

## 世界操作（PC，设计基线§3.1）

| 输入 | 功能 |
|---|---|
| 鼠标右键点地面 | 移动到该处（服务端寻路） |
| 手机点按地面 | 移动（决定Q1：手机只保证启动、连服、点地面移动） |
| S | 停止（`move_stop`，可在项目设置的输入映射中改键） |
| 滚轮 / 按住中键拖动 | 缩放（12–60 米）/ 旋转 |
| Home | 复位镜头（`camera_reset`，可改键） |

目标在风险区、或只能经过风险区到达时，会弹窗确认。I/B/M/J 等面板键位要等对应面板实现后再加。

点击“账号登录”进入账号界面，地址默认 `ws://127.0.0.1:8080/v1/ws`；服务端需配置数据库才开放该接口。登录凭证只保存在内存中，断线后客户端按退避时间（0.5 秒起，最长 30 秒）自动重连并用凭证恢复会话；被别处登录踢下线（关闭码 4001）时不会重连。协议见 [`shared/protocol/v1.md`](../shared/protocol/v1.md)。

手机上的 `127.0.0.1` 指手机自身：真机测试需使用开发电脑的局域网地址，并按服务端说明显式配置 LAN 监听、配置防火墙，勿把无鉴权开发端口暴露公网。开发用 HTTP 不是正式生产传输方案。

## 自动测试

首次运行前先导入一次，让 `Protocol`、`WsClient` 等全局类生效：

```sh
godot --headless --editor --path client --import --quit
```

不需要启动服务器：

```sh
godot --headless --path client --script res://tests/test_bootstrap.gd
godot --headless --path client --script res://tests/test_protocol.gd
godot --headless --path client --script res://tests/test_account.gd
godot --headless --path client --script res://tests/test_world.gd
```

实际端到端网络测试（必须先启动 Go 服务）：

```sh
godot --headless --path client --script res://tests/smoke_health.gd
# 测试自定义开发地址：
godot --headless --path client --script res://tests/smoke_health.gd -- http://127.0.0.1:8080
```

账号与 WebSocket 端到端测试（服务端须配置数据库）：

```sh
godot --headless --path client --script res://tests/smoke_ws.gd -- ws://127.0.0.1:8080/v1/ws
```

它会依次注册、登录、建角、列出角色、进入江湖，走 3 米并等快照确认到达，确认进入风险区会被要求确认，然后用第二个连接登录同一账号，确认第一个连接以关闭码 4001 被踢下线、新连接可以接管原角色并离开江湖。

人工视觉检查（需要窗口，不在 CI 中运行）：

```sh
godot --path client --resolution 1280x800 --script res://tests/capture_world.gd -- /tmp/shots ws://127.0.0.1:8080/v1/ws
```

失败退出码为 1。无网络单元测试覆盖场景实例化、按钮连接、地址校验、正常响应、错误 JSON、非 200 与连接错误；网络 smoke 通过同一个客户端按钮处理逻辑发起真实 HTTP 请求。

## 目录

- `scenes/`：场景；`main.tscn` 为启动页，`account.tscn` 为账号界面。
- `scripts/`：类型化 GDScript；`net/` 下是协议编解码（`Protocol`）和 WebSocket 连接（`WsClient`）；`world/` 下是快照插值（`SnapshotBuffer`）、灰盒地图生成（`MapBuilder`）和镜头（`CameraRig`）。
- `export_presets.cfg`：macOS、Windows、Android、iOS 四个导出预设，导出时排除 `tests/`。
- `assets/`：未来运行时资源，概念图在仓库根目录另存。
- `tests/`：无需第三方测试插件的 headless 脚本。

## 四端边界

目标为 Windows、macOS、iOS、Android。当前使用 Compatibility 渲染器降低空工程验证门槛；正式 3D 战斗阶段需按移动端预算评估 Mobile 渲染器，不能据此宣称画质或性能达标。

导出状态（M3）：
- macOS：本机已导出并启动验证（内置 ad-hoc 签名，未公证）。
- Windows、Android（调试包）：由 `.github/workflows/export.yml` 在 CI 导出，结果以 GitHub 实际运行为准。
- iOS：即使只导出 Xcode 工程，也需要 Apple 开发者团队 ID（`application/app_store_team_id`），尚未配置。
- 均未做真机测试。项目已开启 ETC2/ASTC 纹理压缩（arm64 和移动端导出必需）。

未提供虚假的导出证书与模板：实际导出需安装匹配的 Godot export templates；Android 需 SDK/JDK 并启用 INTERNET 权限，iOS 需 macOS/Xcode 与签名。移动端对明文 HTTP、局域网访问等限制需在实际导出配置中验证，生产环境应使用合规 HTTPS。尚未完成四端导出、真机测试、战斗、资产同步与切后台。

官方参考：
- [HTTPRequest API](https://docs.godotengine.org/en/stable/classes/class_httprequest.html)
- [命令行运行与 headless](https://docs.godotengine.org/en/stable/tutorials/editor/command_line_tutorial.html)
