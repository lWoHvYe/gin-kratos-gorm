# plan — AC-27A-STATE-01（实现变更）

| 项 | 值 |
|---|---|
| change_id | `AC-27A-STATE-01` |
| 级别 | **实现变更**（SPEC-0001 v1.6 已批准 `state = result` 谓词；不在契约主题内，不新建 Spec/ADR） |
| 权威 Spec | `specs/0001-fulfillment-quantity-split/spec.md` v1.6（2026-11-28 批准；AC-27A 第 5/6 条、IF；§16.2 `AC-27A-STATE-01`） |
| 关联 ADR | `docs/adr/0005-fulfillment-quantity-split.md` Rev 1.3 §21（S2 聚合/关闭口径；本变更不改 ADR） |
| base | `dee75b0`（v1.6 落盘后） |
| 阶段 | 代码（Spec 已批准，AC 状态以审查者代码阶段裁决为准） |

## 1. 问题与目标

SPEC-0001 v1.6 AC-27A 第 5 条要求“有效 PRE = `result = PRE_SHIP_REVOKED` **且 `state = result`**”；
实现 `internal/services/order/fulfillment_groups.go` 的 `loadFulfillmentGroups` 只用
`EXISTS(... a.result = 'PRE_SHIP_REVOKED')` 派生 `PreShipAdmission`，未检查 `state`。
因此 `result=PRE, state≠PRE` 的异常/存量行会被关闭谓词当成“已关闭”，与 v1.6 及 SPEC-0002
v1.11 S20/AC-69 的 `state = result` 成功谓词不一致（fail-open）。

**目标**：把派生条件改为 `result = 'PRE_SHIP_REVOKED' AND state = result`，方向 fail-closed。

## 2. 改动面（baseline_impact）

| 项 | 影响 |
|---|---|
| 代码 | `internal/services/order/fulfillment_groups.go`：EXISTS 加 `AND a.state = a.result`；更新字段/查询注释 |
| 代码（注释） | `internal/services/order/settlement.go`：`groupIsClosed` 第 5 条注释补 `state=result` |
| 测试 | `internal/services/order/ac27a_closure_integration_test.go`：新增 `result=PRE, state=PENDING_REMOTE` → `PreShipAdmission=false`、`groupIsClosed=false` 用例 |
| schema/迁移 | **无**（0016 已有 `result`/`state` 列；无新列/索引） |
| 写路径 | **无**（只改读模型派生；所有 PRE 写入路径均 `result=state=PRE_SHIP_REVOKED`，见 `refund_admission.go:523/551/557/952`） |
| S1/S3/S5/S6 | 不改；`groupIsClosed` 仍是唯一关闭权威 |
| 跨服务 | 不改协议/事件；不触 `tcc_*` 的 `admission_result` 镜像（不同字段） |
| INV | 不变量不变：INV-45（PRE 终态语义）、INV-51（`REVIEW_REQUIRED` 不关闭）、INV-38（读模型投影）；本变更只把“什么算有效 PRE”对齐到已批准 Spec |
| 行为差异 | 仅 `result=PRE 且 state≠result` 的行：由“关闭”变为“未关闭”。当前写入路径不产生该组合；若生产存在（legacy/人工），变化方向为 fail-closed（不误关组） |

## 3. AC → 检查

| AC/条款 | 检查 |
|---|---|
| AC-27A 第 5 条（有效 PRE） | `TestAC27AAdmissionResultDerivation`：`result=PRE,state=PRE` → closed；`result=PRE,state=PENDING_REMOTE` → open |
| AC-27A 第 6 条（未关闭面） | 同一测试的 `REVIEW_REQUIRED`/`POST`/`PENDING`/`missing` 用例保持 open |
| AC-27A IF（告警） | 读模型只判关闭，不新增告警路径；异常行按第 6 条“未关闭”，告警归 ops/AC-27-NOWB-01 |
| AC-27（S2 聚合） | 2 组用例 `TestAC27A...`/`TestGroupShipmentProjection` 不回归 |

## 4. stop_conditions

1. 若发现任何**当前写入路径**会写出 `result=PRE` 且 `state≠PRE` 的行 → 停止，回 Spec/ADR 裁决。
2. 若改动需要 schema/迁移/新字段 → 停止（超出本 change）。
3. 若触到 `T5.2/T5.3` 证据或 PAY-SETTLE T5 范围 → 停止并请示人类。
4. 若集成测试无法运行（无 MySQL）→ 记录 SKIP/未覆盖，不得声称 PASS。
5. 若 `go build`/`go vet` 或既有 AC-27A 用例回归失败 → 停止，不夹带修复。

## 5. 交付与审查

- 证据：`specs/0001-fulfillment-quantity-split/evidence/AC-27A-STATE-01/evidence.md`（绑定精确 HEAD、测试日志、【测试范围】【未覆盖范围】【取舍】）。
- 提交尾注：`change_id: AC-27A-STATE-01` / `Actor: lead` / `Reviewed-by: pending`。
- 代码阶段由独立 reviewer 审；PASS 前不标 AC-VERIFIED、不改 §17/§15。
