# 江湖 · jianghu

[English](README.md) | 简体中文

原创武侠多人在线养成游戏：即时战斗、武学传承、门派与生活职业，构建一个人也能持续游玩的持久江湖世界。

**当前阶段：策划＋工程启动骨架，已完成第一批规则模块，尚不是可玩的多人游戏。**

- 客户端：Godot 4＋类型化GDScript。
- 服务端：Go（Golang）。
- 交付目标：Windows、macOS、iOS、Android，四端均为必需；目前尚无平台通过发布验收。
- 所有结果由服务端权威裁定；持久资产存入PostgreSQL。

## 仓库结构

```text
client/              Godot工程、启动场景、脚本与客户端测试
server/              Go服务端：配置、HTTP诊断接口、规则包与单元测试
shared/data/         服务端加载的版本化测试数据（schema_version 1）
shared/protocol/     客户端/服务端协议说明
assets/concept-art/  场景、角色概念图与生成提示词
docs/                策划快照、技术决策、字段字典与验证记录
scripts/             本地检查与数据导出脚本
.github/workflows/   持续集成检查
```

## 快速开始

需要Go 1.25+、Python 3及Godot 4标准版。Go第三方依赖：`pgx`（PostgreSQL）、`coder/websocket`、`golang.org/x/crypto`。数据库可选：不设置`JIANGHU_DATABASE_URL`时服务端只提供诊断接口（见[server/README.md](server/README.md)）。

```sh
# 从仓库根目录
cd server
go run ./cmd/jianghu-server
```

另一终端：

```sh
curl http://127.0.0.1:8080/healthz
# 返回含 status=ok 的JSON；仅证明基础HTTP服务运行
```

在Godot中导入`client/project.godot`，运行主场景，点击健康检查按钮。手机调试时服务端显式监听局域网地址，客户端填写电脑的局域网IP；手机的127.0.0.1指手机自己。不要把无认证的骨架直接暴露公网。

```sh
# Go测试、数据导出检查、导入文件校验与策划模型校验
./scripts/check.sh
# Godot headless测试见 client/README.md
```

## 当前已有内容

| 部分 | 状态 |
|---|---|
| 服务端传输 | HTTP `/healthz`和`/version`；配置数据库后另有WebSocket `/v1/ws`，用于账号、角色、进入/离开江湖、移动与快照（[协议v1](shared/protocol/v1.md)） |
| 测试数据 | `shared/data/`：物品、配方、经济、招式、传承路线、地图与时间参数；其中4份由`scripts/export_data.py`从策划快照生成，与源不一致时CI报错 |
| 服务端规则包 | `gamedata`（加载与交叉校验数据）、`growth`（属性按0.1结算）、`crafting`（制造、磨损、维修、回收）、`economy`（NPC有限采购订单）、`ledger`（堆叠、格数与负重检查） |
| 资产账本 | `store`：PostgreSQL迁移、请求幂等（同一请求ID重放返回首次结果）、物品位置唯一、钱币不为负、容器按序加锁 |
| 验证证据 | Go规则复现了策划的Python演算：成长案例18/18、装备物料账全流程、经济各场景快照逐字段一致；50个并发请求下同一物品只被拾取一次、钱币不会为负。同一账号第二处登录会踢掉第一个连接，同号只有一个角色在江湖。见[M0](docs/verification/m0-data-and-rules.md)、[M1](docs/verification/m1-persistence.md)、[M2](docs/verification/m2-accounts-protocol.md)、[M3](docs/verification/m3-world-movement.md)验证记录 |
| 客户端 | 启动页、账号界面和六区块灰盒世界：右键（手机点按）地面移动、可见其他玩家、风险区确认、镜头缩放旋转 |
| 世界 | `world`：一个20Hz权威循环；服务端规划路径并按4米/秒移动，客户端只发目标点；普通路线绕开风险区；离开时保存位置 |

尚未实现：战斗、游戏内物品操作、交易、运输合同、TLS与部署。安装包：macOS已在本机导出并启动验证；Windows和Android由CI导出；iOS需要Apple开发者团队ID。所有策划数值都是测试参数，16项可玩验收全部待执行。

## 开发计划

按设计基线的阶段门推进，每个阶段都以测试证据结项。

1. **M0 数据与规则**：已完成。
2. **M1 持久化与事务**：已完成。
3. **M2 账号与实时协议**：已完成。
4. **M3 世界与移动**：已完成。
5. **M4–M7 玩法**：战斗与死亡、两节点仓储与物流、生活技能与装备、任务与传承。
6. **M8 首个可玩闭环验收（G1）**：16项逐项实机执行并留存证据。

## 文档与图片

- [文档导航](docs/README.md)
- [技术基线：Godot＋Golang](docs/decisions/0001-technical-baseline.md)
- [设计基线与技术基线对齐](docs/decisions/0002-baseline-reconciliation.md)
- [字段字典](docs/data/field-dictionary.md)与[状态机](docs/data/state-machines.md)
- [现有策划快照](docs/design/v0.44/游戏策划设定.md)
- [概念美术](assets/concept-art/README.md)

![新手村远景](assets/concept-art/starter-village-wide-v1.png)

## 开源与贡献

采用[MIT许可证](LICENSE)。第三方依赖及素材保留各自许可证；不得提交密钥、真实玩家信息、签名证书和来源不明的素材。AI概念图来源与权利边界见概念美术说明。提交前运行本地检查，修改规则时同时更新测试与相关文档。
