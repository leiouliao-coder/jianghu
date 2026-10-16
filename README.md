# 江湖 · jianghu

原创武侠多人在线养成游戏。即时战斗、武学传承、门派与生活职业，构建一个人也能持续游玩的持久江湖世界。

**当前阶段：策划＋工程启动骨架，尚不是可玩的多人游戏。**

- 客户端：Godot 4＋类型化GDScript。
- 服务端：Go（Golang）。
- 交付目标：Windows、macOS、iOS、Android，四端均为必需；目前未完成四端发布验收。
- 服务端权威结算，未来以PostgreSQL保存持久资产；当前未接入数据库。

## 仓库结构

```text
client/              Godot工程、启动场景、脚本与客户端测试
server/              Go服务端、配置、HTTP健康接口与单元测试
shared/protocol/     客户端/服务端协议说明
shared/data/         将来共用的策划导出数据约定（非已实现运行规则）
assets/concept-art/  场景、角色概念图与生成提示词
docs/               策划、技术决策、导入清单与验证边界
scripts/             本地检查脚本
.github/workflows/   持续集成检查
```

## 快速开始

需要Go 1.25+、Python 3及Godot 4标准版。无需Go第三方依赖，无需先启动数据库。

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
# 检查Go、策划数据、导入文件校验
./scripts/check.sh
# Godot headless测试见 client/README.md
```

## 文档与图片

- [文档导航](docs/README.md)
- [当前技术基线：Godot＋Golang](docs/decisions/0001-technical-baseline.md)
- [现有策划快照](docs/design/v0.44/游戏策划设定.md)
- [概念美术](assets/concept-art/README.md)

![新手村远景](assets/concept-art/starter-village-wide-v1.png)

## 实现边界

当前仅建立工程结构和健康握手，不含账号、角色持久化、实时移动/战斗同步、交易、送镖、数据库或部署。策划数值仍为测试参数，16项可玩验收全部待执行。历史策划中Unity/.NET等旧技术提案由当前ADR覆盖，不能误当现行技术栈。

## 开源与贡献

采用[MIT许可证](LICENSE)。第三方依赖及素材保留各自许可证；不得提交密钥、真实玩家信息、签名证书和来源不明的素材。AI概念图来源与权利边界见概念美术说明。提交前运行本地检查，修改规则时同时更新测试与相关文档。
