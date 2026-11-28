# tasks — AC-27-OPS-CONV-01

- [x] T1 新增 `TestOpsReplayNowbRearmConvergesToPre`（NOWB ①：REVIEW → re-arm → CANCELLED → PRE → 组关闭；`orders.status` 不变；无 Payment）。
- [x] T2 复用 OPS-P5b：真 consumer/DLQ（D1）与 TCC re-arm 重驱到 PRE/POST（D5）在 HEAD 复跑。
- [x] T3 验证：`go build ./...`、`go vet ./internal/services/order`、定向 4 用例（MySQL+NATS）、order 全包（MySQL+NATS）。
- [ ] T4 证据：`evidence.md` 绑定提交 HEAD，写【测试范围】【未覆盖范围】【取舍】。
- [ ] T5 提交（`Reviewed-by: pending`）并交独立 reviewer；由审查者裁决 `AC-27-NOWB-01`/`AC-27-LIVE-01` 是否可关闭。
