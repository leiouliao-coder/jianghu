# M0 · G0 数据与规则 验证记录

日期：2026-10-16。范围：开发计划M0（`.omc/plans/jianghu-dev-plan-v1.md`）。**这不是游戏验收**：16项可玩验收仍全部待执行，本阶段不声称游戏可玩。

## 产物

- [ADR 0002](../decisions/0002-baseline-reconciliation.md)：记录基线§4.1的Unity/.NET写法与ADR 0001的冲突，说明处理方式（以ADR 0001为准，基线中与引擎无关的规则照常执行），以及测试世界重置规则。
- `shared/data/`：6份schema_version 1的测试数据（4份生成、2份人工维护）；`scripts/export_data.py --check`已接入`scripts/check.sh`。
- [字段字典](../data/field-dictionary.md)、[状态机](../data/state-machines.md)。
- Go领域包（无第三方依赖）：`gamedata`（加载与交叉校验）、`growth`（五属性0.1结算）、`crafting`（装备生命周期）、`economy`（NPC有限采购订单簿）、`ledger`（堆叠、格数、负重预检）。

## 对照证据

| Go测试 | 对照的源 | 结果 |
|---|---|---|
| `growth.TestAcceptedGrowthCases` | `growth_cases_v37.json`全部18例（即`verify_growth_v37.py`的输入） | 18/18通过 |
| `crafting.TestEquipmentLifecycleMatchesLedger`、`TestRejectedOperationsChangeNothing` | `equipment_ledger_v41.json`：两件装备完整生命周期、重复回收、重放，每件5个拒绝分支 | 通过 |
| `economy.TestEconomyScenariosMatchGolden` | `economy_v44_run.json`：A、B、三个负例、C（24小时与维护）、D（离线交付）、滚动额度，以及价格表与生命周期轨迹 | 全部快照逐字段一致（经JSON归一化后比较） |
| `economy.TestNegativeDeliveries` | 交付他人物品、缺失物品、磨损物品、未知订单、重复订单ID、消耗玩家物品 | 均被拒，订单簿不变 |
| `ledger.*` | 基线§3.5的重量、堆叠和负重示例（臂力20负重100） | 通过 |
| `gamedata.TestLoadRejectsBrokenData` | 5类损坏数据 | 均被拒绝 |

变异检查：临时把预算池每小时补充改为+1，`TestEconomyScenariosMatchGolden`随即失败；恢复后通过。这说明黄金比较能捕捉规则漂移。

已执行命令（Go 1.25.6 darwin/arm64）：`./scripts/check.sh`退出码0，包括`go test -race ./...`、`go vet ./...`、导入SHA校验28份、数据导出检查4份、成长/传承/物料账/经济Python演算。

## 未验证与限制

- 经济演算的“750条断言”是Python模型中不变式的累计调用次数，Go没有复现这个计数；Go改用快照逐字段一致作为等价证据。
- Go的`economy`/`crafting`是串行领域模型：请求幂等、持久化和并发争用属于M1。测试中的钱包和物品表是测试脚手架，不是正式账本。
- `moves.json`尚未包含范围、角度等几何数据（M4补齐）；`world.json`中的连线没有实际距离和寻路数据（M3）。
- 生活技能结算（基线§3.4）只录入了参数，结算逻辑在M6实现；成长例03中的技能增长在测试里单独计算，只用于验证“属性锁定不影响技能增长”。
- GitHub Actions的远程运行结果以GitHub实际状态为准。
