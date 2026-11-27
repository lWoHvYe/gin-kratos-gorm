# v3.0 adoption — process only

Status: trial branch applied (`d90d850`); merge only after reviewer + human approval.

## Scope

- v3.0 只改变角色/流程/审查/证据；**不改 Spec 布局**。
- `docs/specs/0001/0002` 保持原样：不迁移、不复制、不改路径。
- `specs/` 现有布局与 README 保持原样；不新建 per-change 第二份 Spec。
- `specs/sdd-struct-align-01` 保持原样；不标 historical、不做 AC 位移。
- `Spec.md`、`specs/README.md`、`docs/specs/README.md` 本轮不改。
- ADR-0009 保持原样；Spec 权威/迁移是另案。

## 采用

1. 应用 `CONSTITUTION.v3.md`、`AGENTS.v3.md` 的 process 部分；
2. 不执行任何 Spec 文件移动/README 切换；
3. 记录 adoption 的 change_id、base/head、reviewer/human 裁决；
4. 跑两个真实变更，按 `PILOT.md` 记录指标。

## Rollback

- revert v3.0 process commit；没有文件迁移需要反向恢复。
