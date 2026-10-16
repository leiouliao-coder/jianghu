# ADR 0001：当前技术与平台基线

状态：按最新项目要求建立的工程起点，尚非完整游戏。

- 客户端：Godot 4标准版，类型化GDScript；不使用Unity或Godot .NET客户端。
- 服务端：Go（Golang），不是Godot服务端，也不是C#/.NET服务端。
- 必须交付：Windows、macOS、iOS、Android。移动端已从“后续评估”变为必需目标；四端尚未完成构建或真机验收。
- 数据库目标：PostgreSQL，当前骨架未接入；不假装具备资产持久化。
- 网络：客户端与Go服务端采用明确版本化消息。当前仅HTTP健康握手，不是游戏同步协议，不使用Godot高层RPC假装直连Go。
- 服务端权威：移动、命中、伤害、掉落、库存、交易与货币必须由服务端裁定；当前均未实现。
- 初期模块化单仓库，不预先拆大量微服务。正式网络传输、导航与碰撞数据导出留到原型实测，不能将客户端位置直接当真。

## 与历史策划的关系

`docs/design/v0.44/`为历史策划快照：规则已确认、授权代拟与测试参数的边界保持不变；其中“Unity/.NET”“移动端以后评估”等技术描述由本ADR覆盖，不改写成历史上已被确认的选择。产品硬规则不因工程初始化而变化。

## 版本和验证

首轮使用可用的Godot 4稳定版进行导入、启动及headless检查，实际版本记入工程验证记录。Go使用go.mod及CI声明版本。正式发布前锁定引擎补丁版、工具链和插件版本，并分别完成四端导出、签名与真机测试。

官方参考：[Godot导出](https://docs.godotengine.org/en/stable/tutorials/export/index.html)、[Godot CLI](https://docs.godotengine.org/en/stable/tutorials/editor/command_line_tutorial.html)、[Go模块](https://go.dev/doc/modules/gomod-ref)。
