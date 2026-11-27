# REV-PAY-SETTLE-01-T5-P3A — ADR-0011 Phase 3a（Order 回执 + ack consumer）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `migrations/order/0024_refund_release_receipts.sql`；`internal/services/order/refund_release_ack_consumer.go`；`cmd/order/main.go` 启动接线；`settlement_integration_test.go` 的 `orderTestDB` 0024；`refund_release_ack_consumer_test.go`。**不审** Phase 3b Payment-last、legacy worker、T5.2/3/5/6/7。 |
| base | **`f27c17c`** |
| head | **工作树 @ `f27c17c`（未提交）**：新 `0024`、ack consumer、2 个测试；`cmd/order/main.go` +3；`orderTestDB` drop/apply 0024。**无** Inventory/api 改动。 |
| spec_version | **SPEC-0002 v1.11**（AC-63/INV-46） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1**；**ADR-0011**（Phase 1/2 已 PASS） |
| baseline_impact | **有**：Order 回执表 additive；ack consumer 推进 PENDING；无 Payment/legacy worker/release producer；AC-63 仍不升级。 |
| INV | **INV-46**、**AC-63**（Payment 前 RELEASED 的前置回执）；AC-63 仍不升级。 |
| verification | **命令实测**：`TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestT5RealMigration\|TestRefundReleaseAck' -count=1 -v` → **PASS（4.146s；23 个真实迁移）**；`go vet ./internal/services/order ./cmd/order` → 0；`git diff f27c17c -- internal/services/inventory api` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（Phase 3a）**：回执表 additive/CHECK 正确；ack 只推进 PENDING、同态 no-op、终态不降级、FAILED 记 last_error、未知/order 不匹配 fail-closed；无 Payment/legacy worker/release producer；**Phase 3a 可合入；Phase 3b Payment-last 未做；AC-63 仍未验证**。0 阻断；NB-1…NB-6。 |

---

## 0. 结论

1. **迁移 0024 additive**：`refund_release_receipts(release_id PK, refund_id UNIQUE, order_no, state PENDING|RELEASED|FAILED, last_error, version, lease_owner/until, timestamps)`；CHECK 只允许 3 态；`refund_id` 唯一对应一个 `release_id`；无破坏性 DDL。
2. **ack consumer**：`RunRefundReleaseAckConsumer` 订阅 `inventory.refund_release_completed`（durable `order-refund-release-ack`）；`applyRefundReleaseAck` 只把 `PENDING→RELEASED/FAILED`；同态重复 no-op（version 不增）；`current != PENDING` → fail-closed（终态不降级，也不升级）；order 不匹配 → fail-closed；未知 receipt → `NotFound`。
3. **FAILED**：`last_error` 写 ack error（空则默认、截 512）；`RELEASED` 清 last_error；lease 清空。
4. **无 Phase 3b**：无 Payment、无 `legacy_resolution_worker` 改动、无 release request producer；`internal/services/inventory`/`api` 零改动。
5. **启动/迁移**：`cmd/order` 启动 ack consumer；`orderTestDB` drop/apply 0024；`TestT5RealMigration` 现 23 个真实迁移文件。
6. **AC-63 仍未验证**：Phase 3a 只是回执，不是 Payment/`EXPIRED_REFUND` 全链路；AC-63 表保持 `未验证`。

---

## 1. 审查输入与方法

- 读：0024、ack consumer、`cmd/order` diff、`orderTestDB` diff、ack tests、`outbox`/receipt 表。
- 跑：T5 真实迁移 + ack 测试、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestT5RealMigration|TestRefundReleaseAck' -count=1 -v
--- PASS: TestRefundReleaseAckAdvancesReceipt (0.79s)
--- PASS: TestRefundReleaseAckFailedAndUnknown (0.81s)
--- PASS: TestT5RealMigrationSchemaAndAC60Backfill (2.43s)
PASS  (4.146s)

$ go vet ./internal/services/order ./cmd/order   # 0
$ git diff f27c17c -- internal/services/inventory api | wc -l   # 0
```

---

## 3. 迁移 0024

- `release_id` PK、`refund_id` UNIQUE、`order_no`、`state`（CHECK PENDING/RELEASED/FAILED）、`last_error`、`version`、`lease_owner/lease_until`、`created_at/updated_at`；additive。
- `refund_id UNIQUE` 与 `release_id=rr:<refund_id>` 的确定性 id 一一对应；`insertRefundReleaseReceiptTx` 用 `ON DUPLICATE KEY UPDATE release_id = release_id` 做幂等。
- **NB-1**：若调用方传入与既有 `refund_id` 不同的 `release_id`，`ON DUPLICATE` 会命中 `refund_id UNIQUE` 并**静默 no-op**，新 `release_id` 没有回执；Phase 3b 应在 insert 后校验 `release_id` 一致，或 insert 时检测冲突 fail-closed（确定性 `rr:<refund_id>` 下正常不会发生）。

---

## 4. ack consumer 状态机

- `applyRefundReleaseAck`：校验 identity/status；`db.WithTx`；`SELECT ... FOR UPDATE`；not found → `NotFound`；order mismatch → `Failed`；`current == state` → nil（同态 no-op，version 不变）；`current != PENDING` → `Failed("already terminal")`；PENDING → `UPDATE ... WHERE state=PENDING`，version+1、lease 清空。
- `RELEASED`：last_error 清空；`FAILED`：last_error=ack error（空则默认、截 512）。
- 终态语义：RELEASED→FAILED、FAILED→RELEASED 都 fail-closed（不降级，也不升级）；同态重复 no-op。
- **NB-2**：未知 receipt 返回 `apperr.NotFound`；consumer 是否按 permanent DLQ、ack 丢失后的 ops 处置未在测试中覆盖（测试直接调 `apply`）。
- **NB-3**：无 order mismatch / 终态降级/升级的专门测试；`TestRefundReleaseAckFailedAndUnknown` 只覆盖 FAILED + unknown。
- **NB-4**：直接调 `applyRefundReleaseAck` 的测试绕过了 Phase 1 的 `ValidateJSON`（release_id=rr:<refund_id> 等）；生产 consumer 有校验，测试未覆盖。

---

## 5. 启动接线 / durable

- `cmd/order` 新增 `workers.Go(order.RunRefundReleaseAckConsumer(...))`；`RunRefundReleaseAckConsumer` 订阅 `events.RefundReleaseCompletedTopic`，durable `order-refund-release-ack`；ack consumer 与旧 consumer 并行启动。
- **NB-5**：启动接线没有集成测试（只 `go vet`）；durable subject 的命名/重复启动行为依赖既有 `consumer.Subscribe`。

---

## 6. 无 Phase 3b / 边界

- 无 Payment 调用、无 `legacy_resolution_worker` 改动、无 release request producer；`internal/services/inventory`/`api` 零改动；`git diff f27c17c -- internal/services/inventory api` = 0。
- Phase 3b 才写 release + PENDING 回执 + Payment-last + `EXPIRED_REFUND`；本 Phase 只有回执表和 ack consumer。
- **NB-6**：`insertRefundReleaseReceiptTx` 没有直接测试；Phase 3b 才会用，Phase 3a 只交付函数。

---

## 7. AC-63

- Phase 3a 只是回执；没有 Payment/`EXPIRED_REFUND` 全链路；AC-63 保持 **未验证**；Phase 3b/T5.2/3/5/6/7 未做。

---

## 8. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | `insert` 的 `ON DUPLICATE` 在 `refund_id` 冲突不同 `release_id` 时静默 no-op；Phase 3b 应校验一致性/fail-closed | `insertRefundReleaseReceiptTx` |
| NB-2 | 未知 receipt 返回 `NotFound`；consumer DLQ/ack 丢失的 ops 处置未测试 | `applyRefundReleaseAck` |
| NB-3 | 无 order mismatch / 终态降级/升级测试 | ack tests |
| NB-4 | 直接调 `apply` 绕过 Phase 1 validator | ack tests |
| NB-5 | 启动接线无集成测试 | `cmd/order/main.go` |
| NB-6 | `insertRefundReleaseReceiptTx` 无直接测试（Phase 3b 才用） | 新函数 |

---

## 9. 裁决

**PASS（Phase 3a）。** 回执表 additive/CHECK 正确；ack consumer 只推进 PENDING、同态 no-op、终态不降级、FAILED 记 last_error、未知/order 不匹配 fail-closed；启动接线/durable 已接；`orderTestDB`/真实迁移覆盖 0024；无 Payment/legacy worker/release producer；无 Inventory/api 改动。

**Phase 3a 可合入；Phase 3b Payment-last 未做；AC-63 仍未验证；NOT RELEASE。**

---

## 10. 复现命令（只读）

```bash
git rev-parse HEAD   # f27c17c...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestT5RealMigration|TestRefundReleaseAck' -count=1 -v
go vet ./internal/services/order ./cmd/order
git diff f27c17c -- internal/services/inventory api | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
