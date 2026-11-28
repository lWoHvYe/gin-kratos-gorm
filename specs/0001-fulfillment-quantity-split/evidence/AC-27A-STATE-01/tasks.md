# tasks — AC-27A-STATE-01

- [ ] T1 实现：`fulfillment_groups.go` 的 `PreShipAdmission` EXISTS 加 `AND a.state = a.result`；同步 struct/查询注释（v1.6 有效 PRE）。
- [ ] T2 注释：`settlement.go` `groupIsClosed` 第 5 条补 `state=result`（不改逻辑）。
- [ ] T3 测试：`ac27a_closure_integration_test.go` 新增 `result=PRE, state=PENDING_REMOTE` 负例（`PreShipAdmission=false` / `groupIsClosed=false` / `effectiveGroupStatus` 保持原状态）。
- [ ] T4 验证：`go build ./...`、`go vet ./internal/services/order`、`go test -race` 定向（`TestAC27A`、`TestGroupShipmentProjection`、`TestAdmitRefund` 子集）+ 全包 `go test ./internal/services/order -count=1`（带 TEST_MYSQL_DSN）。
- [ ] T5 证据：`evidence.md` 绑定 `git rev-parse HEAD`、贴日志、写【测试范围】【未覆盖范围】【取舍】。
- [ ] T6 提交（`Reviewed-by: pending`）并交独立 reviewer 代码阶段审查。

> 批次：T1–T3 为一个实现批；T4 验证后再交审。若 T4 失败，停在本批，不带病提交。
