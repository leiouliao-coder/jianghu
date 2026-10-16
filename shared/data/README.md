# 共用策划数据

服务端加载的版本化测试数据（schema_version 1，`status: test`），由`server/internal/gamedata`加载并交叉校验。字段与单位见[字段字典](../../docs/data/field-dictionary.md)，权威源规则见[ADR 0002](../../docs/decisions/0002-baseline-reconciliation.md)。

- 生成文件：`recipes.json`、`economy.json`、`moves.json`、`routes.json`。源在`docs/design/v0.44/`，用`python3 scripts/export_data.py`重新生成，不要手改；`./scripts/check.sh`会检查它们是否与源一致。
- 人工维护：`items.json`、`world.json`，文件内注明来源章节。

这些仍是测试参数，不是最终平衡；服务端是唯一的权威加载方，客户端不得以本地副本裁定结果。
