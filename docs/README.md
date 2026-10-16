# 文档导航

## 先读

1. [当前技术与平台决策](decisions/0001-technical-baseline.md)：Godot＋Golang，四端必须交付。
2. [完整设计执行基线](design/v0.44/完整设计执行基线_v0.44.md)：十二组系统、默认规则和实施门槛。
3. [24项设定汇总](design/v0.44/24项已确认设定与待细化汇总.md)。
4. [策划主文档](design/v0.44/游戏策划设定.md)：含历史决策与版本演进。
5. [实施与验证清单](design/v0.44/待讨论问题.md)：文件名沿用，当前不再逐项问答。

## 实施数据（开发计划M0起）

- [ADR 0002：设计基线与技术基线对齐](decisions/0002-baseline-reconciliation.md)
- [字段字典](data/field-dictionary.md)与[状态机](data/state-machines.md)：`shared/data/`的格式与规则。
- [M0验证记录](verification/m0-data-and-rules.md)、[M1验证记录](verification/m1-persistence.md)、[M2验证记录](verification/m2-accounts-protocol.md)、[M3验证记录](verification/m3-world-movement.md)、[M4验证记录](verification/m4-combat-death.md)
- [实时协议 v1](../shared/protocol/v1.md)

## 测试附件

- [剑拳、内功、轻功](design/v0.44/剑拳内功轻功_招式样表_v0.35.md)
- [属性映射与成长案例](design/v0.44/行为—属性映射与成长案例_v0.37.md)
- [两派＋散人传承路线](design/v0.44/transmission_routes_v39.md)
- [装备全生命周期物料账](design/v0.44/equipment_ledger_v41.md)
- [测试价格与经济三场景](design/v0.44/economy_v44.md)
- [首个可玩闭环与16项验收](design/v0.44/first_playable_loop_v42.md)

数值是测试参数，不是最终平衡。本地Python断言只检验模型，不代表多人游戏、反作弊、四端性能或16项游戏验收已通过。所有16项游戏验收仍待实际开发后执行。

## 导入说明

`design/v0.44/`保留现有策划及可复算数据/脚本；仅清理本机绝对工作路径及Markdown行尾空白，源与仓库SHA见[导入清单](import-manifest.json)。历史本机事务大日志、包含旧版本嵌入内容的回滚脚本及ZIP没有直接塞进公开源码仓库；本仓库历史由Git保留。所有新文档链接应使用仓库相对路径。

概念图与原始提示词见[`assets/concept-art/`](../assets/concept-art/README.md)。
