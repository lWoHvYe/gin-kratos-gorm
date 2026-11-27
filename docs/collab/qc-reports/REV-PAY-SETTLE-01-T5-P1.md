# REV-PAY-SETTLE-01-T5-P1 — ADR-0011 Phase 1（契约事件）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/platform/events/events.go`（`legacy_24h` source/reason + ack 事件/struct/status）；`registry.go`（source→reason、ack schema、validator）；`refund_release_test.go`；Inventory `refund_release_consumer_test.go`。**不审** Phase 2 Inventory ack producer、Phase 3 Order receipt/Payment、T5.2/3/5/6/7。 |
| base | **`a9758c7`** |
| head | **工作树 @ `a9758c7`（未提交）**：`events.go` sha256 `eaa8dcb0…`；`registry.go` `fdfc9728…`；events test `da6bdb34…`；Inventory test `8bd4e100…`。**无** `internal/services/order`/`migrations`/`api` 改动。 |
| spec_version | **SPEC-0002 v1.11**（AC-63/INV-46） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1**；**ADR-0011 Accepted**（2026-11-27 人类裁决 A） |
| baseline_impact | **有**：新增 `legacy_24h`/`LEGACY_24H_EXPIRY`、ack 事件/schema/validator；Inventory consumer 可消费；**无** producer/Order receipt/Payment/migration。 |
| INV | **INV-46**（历史解析/24h 收敛）、**AC-63**（COMPENSATING→EXPIRED_REFUND 的前置契约）；AC-63 仍不升级。 |
| verification | **命令实测**：`go test ./internal/platform/events -run 'TestRefundRelease' -count=1 -v` → **PASS（0.100s）**；`go test ./internal/services/inventory -run 'TestRefundRelease' -count=1` → **PASS（0.117s）**；`go vet ./internal/platform/events ./internal/services/inventory` → 0；`git diff a9758c7 -- migrations api internal/services/order` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（Phase 1）**：`legacy_24h` 只新增、旧 source 仍封闭；ack schema/validator 覆盖 release_id/refund_id/order/status/time，FAILED 可表达；Inventory consumer 已接受；无 Phase 2/3 行为；**Phase 1 可合入；Phase 2 Inventory ack 未做；AC-63 仍未验证**。0 阻断；NB-1…NB-3。 |

---

## 0. 结论

1. **source pair 只新增**：`RefundReleaseSourceLegacy24h="legacy_24h"` + `RefundReleaseReasonLegacy24hExpiry="LEGACY_24H_EXPIRY"`；registry `refundReleaseReasons` 只加这一对；旧 `tcc/system_zero/system_shortage/aftersale` 不变；`payment`/`legacy_expired` 未加入，旧 source 仍封闭。
2. **ack schema/validator**：`RefundReleaseCompletedTopic/EventType` + struct（schema_version/release_id/refund_id/order_no/status/items/released_at/error）；validator 要求 `release_id=rr:<refund_id>`、status `RELEASED|FAILED`、`released_at` RFC3339、`order_no` 非空；FAILED 可表达（测试 valid FAILED + error）。
3. **consumer-first 成立**：Inventory consumer 不按 source 分支；registry 接受 `legacy_24h` 后，`handleRefundRelease` 可消费；Order producer 尚未发送（`internal/services/order` 无改动）；测试 `TestRefundReleaseLegacy24hAcceptedByConsumer`。
4. **无 Phase 2/3 提前写入**：无 ack producer、无 `refund_release_receipts`、无 `legacy_resolution_worker` 改动、无 Payment、无 migration；`git diff` 只 events/registry/tests。
5. **测试**：source pair（`TestRefundReleaseSourceReasonPairs` 加 `legacy_24h`）+ ack 正反例（`TestRefundReleaseCompletedContract`）+ Inventory 接受（`TestRefundReleaseLegacy24hAcceptedByConsumer`）全 PASS。
6. **AC-63 仍未验证**：Phase 1 只是契约事件，不是全链路；AC-63 表保持 `未验证`。

---

## 1. 审查输入与方法

- 读：`events.go`/`registry.go` diff、`refund_release_test.go`、Inventory consumer test、ADR-0011 Phase 1 范围。
- 跑：events/inventory 测试、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/platform/events -run 'TestRefundRelease' -count=1 -v
--- PASS: TestRefundReleaseSourceReasonPairs
--- PASS: TestRefundReleaseCoveredGroupAuditIsValidForSingleGroup
--- PASS: TestRefundReleaseCompletedContract (含 5 个 invalid 子测试)
PASS  (0.100s)

$ go test ./internal/services/inventory -run 'TestRefundRelease' -count=1
PASS  (0.117s；新增 TestRefundReleaseLegacy24hAcceptedByConsumer)

$ go vet ./internal/platform/events ./internal/services/inventory   # 0
```

---

## 3. source pair：只新增、旧 source 仍封闭

- `events.go` 新增 `RefundReleaseSourceLegacy24h`/`RefundReleaseReasonLegacy24hExpiry`；registry 只加这一对。
- 旧 source `tcc/system_zero/system_shortage/aftersale` 映射不变；`payment`/`legacy_expired` 未加入（仍被 `RefundReleaseReasonForSource` 拒绝）。
- `TestRefundReleaseSourceReasonPairs` 加 `legacy_24h`，并保留旧 source 的 pair 断言；`TestRefundReleaseCompletedContract` 末尾校验 `RefundReleaseReasonForSource(legacy_24h)`。
- **结论**：只新增，不复用旧 source；旧枚举仍封闭。

---

## 4. ack schema/validator

- `RefundReleaseCompleted`：schema_version、release_id、refund_id、order_no、status、items、released_at、error。
- `validateRefundReleaseCompleted`：schema_version=1；refund_id 非空；`release_id == RefundReleaseID(refund_id)`（`rr:<refund_id>`）；order_no 非空；status ∈ {RELEASED,FAILED}；released_at RFC3339。
- schema required：`schema_version/release_id/refund_id/order_no/status/released_at`；`items`/`error` 可选。
- FAILED：测试用 status=FAILED + error 通过；Order receipt 可据此写 FAILED/`last_error`。
- **NB-1**：`items` 未 required/validated；ADR ack 事件含 items，Order receipt 不存 items，Phase 2/3 若需要逐项 reconciliation 需补。
- **NB-2**：FAILED 的 `error` 可选、未校验；Order `last_error` 可能为空；建议 FAILED 时要求/记录 error。
- **NB-3**：`released_at` 对 FAILED 仍是 required；FAILED 实际没有“release 完成时间”，语义可改为 ack_at/failed_at；不阻断 Phase 1。

---

## 5. consumer-first

- Inventory `handleRefundRelease` 不按 source 分支；registry 接受 `legacy_24h` 后，`RefundReleaseRequested` 可被消费；`TestRefundReleaseLegacy24hAcceptedByConsumer` 断言 release call 1 次、order no 正确。
- Order producer 未发送：`internal/services/order` 无改动；`refund_release.go`/`legacy_resolution_worker.go` 未改；无 outbox producer。
- **结论**：consumer-first 成立：Inventory 先接受，Order 尚未发送。

---

## 6. 无 Phase 2/3 行为

- 无 ack producer（`RefundReleaseCompleted` 只在 events/registry/test 出现，无 outbox append）。
- 无 `refund_release_receipts` 迁移/写入；`migrations` 0。
- 无 `legacy_resolution_worker` 改动；无 Payment 调用；`internal/services/order` 0。
- `git diff a9758c7 -- migrations api internal/services/order` = **0**。
- **结论**：Phase 2/3 未被提前写入。

---

## 7. AC-63

- Phase 1 只是契约事件/validator/consumer 接受；没有 ack 回执、Payment-last、`EXPIRED_REFUND` 全链路。
- AC-63 验收表保持 **未验证**；不升级。
- T5.2/3/5/6/7 未做；Phase 2 Inventory ack 未做；Phase 3 Order receipt 未做。

---

## 8. 边界

- `git diff a9758c7 -- migrations api internal/services/order` = **0**；无 proto/migration/Order 改动。
- 变更仅 events.go/registry.go/events test/inventory test；无生产 Inventory consumer 分支改动（只加测试）。
- 无生成物/二进制残留。

---

## 9. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | ack `items` 未 required/validated；ADR ack 含 items，Order receipt 不存；Phase 2/3 若需逐项 reconciliation 需补 | `RefundReleaseCompleted`；schema required |
| NB-2 | FAILED 的 `error` 可选、未校验；Order `last_error` 可能为空 | validator/schema |
| NB-3 | `released_at` 对 FAILED 仍 required；FAILED 语义可改为 ack_at/failed_at | validator |

---

## 10. 裁决

**PASS（Phase 1）。** `legacy_24h` 只新增、旧 source 仍封闭；ack schema/validator 覆盖 release_id/refund_id/order/status/time，FAILED 可表达；Inventory consumer 已接受；无 ack producer/Order receipt/Payment/migration；测试全 PASS。

**Phase 1 可合入；Phase 2 Inventory ack 未做；AC-63 仍未验证；NOT RELEASE。**

---

## 11. 复现命令（只读）

```bash
git rev-parse HEAD   # a9758c7...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
go test ./internal/platform/events -run 'TestRefundRelease' -count=1 -v
go test ./internal/services/inventory -run 'TestRefundRelease' -count=1
go vet ./internal/platform/events ./internal/services/inventory
git diff a9758c7 -- migrations api internal/services/order | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
