# Godot 客户端基础工程

使用 Godot 4 标准版与类型化 GDScript，无第三方插件。当前只有可运行启动页和 `GET /healthz` 连接检查，**不是游戏原型完成品**。

## 打开与运行

在 Godot 项目管理器导入 `client/project.godot`，按 F6 运行主场景或 F5 运行工程。

从仓库根目录也可执行（将 `godot` 替换为本机 Godot 可执行文件路径）：

```sh
godot --editor --path client
godot --path client
```

先按 `server/README.md` 启动 Go 服务，然后点击“检查服务端”。默认访问 `http://127.0.0.1:8080/healthz`，预期 HTTP 200 且 JSON `status` 为 `ok`。地址栏填写服务器基础地址，不含 `/healthz`。

手机上的 `127.0.0.1` 指手机自身：真机测试需使用开发电脑的局域网地址，并按服务端说明显式配置 LAN 监听、配置防火墙，勿把无鉴权开发端口暴露公网。开发用 HTTP 不是正式生产传输方案。

## 自动测试

不需要启动服务器：

```sh
godot --headless --path client --script res://tests/test_bootstrap.gd
```

实际端到端网络测试（必须先启动 Go 服务）：

```sh
godot --headless --path client --script res://tests/smoke_health.gd
# 测试自定义开发地址：
godot --headless --path client --script res://tests/smoke_health.gd -- http://127.0.0.1:8080
```

失败退出码为 1。无网络单元测试覆盖场景实例化、按钮连接、地址校验、正常响应、错误 JSON、非 200 与连接错误；网络 smoke 通过同一个客户端按钮处理逻辑发起真实 HTTP 请求。

## 目录

- `scenes/`：场景；当前 `main.tscn` 为启动页。
- `scripts/`：类型化 GDScript；当前仅健康检查。
- `assets/`：未来运行时资源，概念图在仓库根目录另存。
- `tests/`：无需第三方测试插件的 headless 脚本。

## 四端边界

目标为 Windows、macOS、iOS、Android。当前使用 Compatibility 渲染器降低空工程验证门槛；正式 3D 战斗阶段需按移动端预算评估 Mobile 渲染器，不能据此宣称画质或性能达标。

未提供虚假的导出证书与模板：实际导出需安装匹配的 Godot export templates；Android 需 SDK/JDK 并启用 INTERNET 权限，iOS 需 macOS/Xcode 与签名。移动端对明文 HTTP、局域网访问等限制需在实际导出配置中验证，生产环境应使用合规 HTTPS。尚未完成四端导出、真机测试、账号、战斗、资产同步、断线重连、后台与切换角色。

官方参考：
- [HTTPRequest API](https://docs.godotengine.org/en/stable/classes/class_httprequest.html)
- [命令行运行与 headless](https://docs.godotengine.org/en/stable/tutorials/editor/command_line_tutorial.html)
