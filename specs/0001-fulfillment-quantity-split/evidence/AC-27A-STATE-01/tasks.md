# tasks — AC-27A-STATE-01

- [x] T1 实现：`fulfillment_groups.go` 的 `PreShipAdmission` EXISTS 加 `AND a.state = a.result`；同步 struct/查询注释（v1.6 有效 PRE）。
- [x] T2 注释：`settlement.go` `groupIsClosed` 第 5 条补 `state=result`（不改逻辑）。
- [x] T3 测试：`ac27a_closure_integration_test.go` 新增 `result=PRE, state=PENDING_REMOTE` 负例（`PreShipAdmission=false` / `groupIsClosed=false` / `effectiveGroupStatus` 保持原状态）。
- [x] T4 验证：`go build ./...`、`make fmt-check vet`、order 全包（MySQL）PASS、`-race -p 1` 定向 PASS。
- [x] T5 证据：`evidence.md` 绑定 `16e472e`，含【测试范围】【未覆盖范围】【取舍】。
- [x] T6 提交（`Reviewed-by: pending`）并交独立 reviewer 代码阶段审查。

> 结果：T1–T6 完成；代码阶段裁决待独立 reviewer（`Reviewer` 写台账与验收表）。
