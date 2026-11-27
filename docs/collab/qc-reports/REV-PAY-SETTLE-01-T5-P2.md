# REV-PAY-SETTLE-01-T5-P2 — ADR-0011 Phase 2（Inventory ack，同事务）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/inventory/refund_release_consumer.go`（路由、legacy ack 同事务、FAILED/retryable、ack 去重）；`refund_release_integration_test.go`（2 个新测试）。**不审** Phase 3 Order receipt/Payment、T5.2/3/5/6/7。 |
| base | **`9853e01`** |
| head | **工作树 @ `9853e01`（未提交）**：consumer.go sha256 `34b0e8d8…`；integration test `eb9b2d78…`。**无** `internal/services/order`/`migrations`/`api` 改动。 |
| spec_version | **SPEC-0002 v1.11**（AC-63/INV-46） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1**；**ADR-0011**（Phase 1 已 PASS） |
| baseline_impact | **有**：Inventory legacy_24h 释放 + ack 同事务；FAILED ack；无 Order receipt/Payment/migration。 |
| INV | **INV-46**（24h 收敛）、**AC-63**（release→Payment 前置）；AC-63 仍不升级。 |
| verification | **命令实测**：`TEST_MYSQL_DSN=... go test ./internal/services/inventory -run 'TestRefundRelease\|TestLegacy24hRelease' -count=1 -v` → **PASS（2.312s）**；`go test ./internal/platform/events -run 'TestRefundRelease'` → **PASS（0.103s）**；`go vet ./internal/services/inventory ./internal/platform/events` → 0；`git diff 9853e01 -- internal/services/order migrations api` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（Phase 2）**：release + ack 同事务；永久失败回滚后写 FAILED ack；可重试失败不写 ack；HELD/CONFIRMED/RELEASED/无行/未知分支与旧语义一致（CONFIRMED 用 reservation quantity）；非 legacy 走旧路径；无 Order/Payment/migration；**Phase 2 可合入；Phase 3 Order receipt/Payment 未做；AC-63 仍未验证**。0 阻断；NB-1…NB-6。 |

---

## 0. 结论

1. **同事务**：`handleRefundReleaseLegacyAck` 在 `svc.write` 单事务内逐项 `applyLegacyReleaseItemTx`，成功后 `appendRefundReleaseAckTx(RELEASED)` 同事务写 Inventory outbox；release 与 ack 同提交/同回滚，不会“release 已提交但 ack 丢失”（除非事务回滚重试）。
2. **永久失败**：release tx 回滚后，`isPermanentReleaseError`（Invalid/NotFound/FailedPrecondition）→ 新事务写 FAILED ack（带 error）→ `consumer.Permanent` DLQ；可重试错误直接返回 err，不写 ack。
3. **分支**：HELD→`releaseTx`；CONFIRMED→`cancelReservationTx(..., r.quantity)`（用 reservation quantity，不用 event quantity，和旧 `TccCancelConfirm` 一致）；RELEASED/无行→跳过；未知 status→Invalid→FAILED ack。
4. **幂等**：重复消息第二次见 RELEASED/无行；ack 按 `event_type+aggregate_id=release_id` 计数跳过；测试重复消息只 1 ack。
5. **非 legacy**：`handleRefundReleaseEvent` 只把 `legacy_24h` 路由到 ack 路径；其他 source 仍 `handleRefundRelease` 旧路径，旧测试全 PASS。
6. **无 Phase 3**：无 Order receipt/Payment/migration；`internal/services/order` 未改；AC-63 仍 `未验证`。

---

## 1. 审查输入与方法

- 读：consumer.go diff、新测试、`outbox_events` schema/`outbox.Append`、旧 `handleRefundRelease` 的 HELD/CONFIRMED/无行/未知分支、`reservationByOrderSku`/`releaseTx`/`cancelReservationTx`。
- 跑：inventory/events 测试、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/inventory -run 'TestRefundRelease|TestLegacy24hRelease' -count=1 -v
--- PASS: TestRefundRelease...（旧路径）
--- PASS: TestLegacy24hReleaseAckInSameTransaction (0.24s)
--- PASS: TestLegacy24hReleasePermanentFailureWritesFailedAck (0.17s)
PASS  (2.312s)

$ go test ./internal/platform/events -run 'TestRefundRelease' -count=1
PASS  (0.103s)

$ go vet ./internal/services/inventory ./internal/platform/events   # 0
$ git diff 9853e01 -- internal/services/order migrations api | wc -l  # 0
```

---

## 3. ack 与 release 同一个 DB 事务

- `handleRefundReleaseLegacyAck` 的 `svc.write` 内：逐项 `applyLegacyReleaseItemTx`（`reservationByOrderSku`/`releaseTx`/`cancelReservationTx` 均带 tx）→ `appendRefundReleaseAckTx(RELEASED)`（`outbox.Append` 同一 tx）。
- 成功路径：release 与 ack 一起提交；不会 release 已提交而 ack 丢失。
- ack append 失败（Internal/非永久）→ 整个 tx 回滚 → 返回 err → 消息重试；不会 release 已提交而 ack 丢失。
- **NB-1**：`TestLegacy24hReleaseAckInSameTransaction` 只断言最终 reservation/ack 都存在，**没有故障注入**证明“ack 失败时 release 回滚”；代码 inspection 已证明同 tx，测试可加强。

---

## 4. 永久失败 / 可重试失败 / FAILED error

- 永久：`isPermanentReleaseError` = InvalidArgument/NotFound/FailedPrecondition；release tx 已回滚；新 `svc.write` 写 FAILED ack（`truncateReleaseError`）→ `consumer.Permanent` → DLQ。
- 可重试：`isPermanentReleaseError` false（Conflict/Unavailable/Internal）→ 直接 return err，不写 ack；consumer 重试。
- FAILED ack 含 `Error`（测试断言非空）；`released_at` 用 ack 时间。
- **NB-2**：`TestLegacy24hReleasePermanentFailureWritesFailedAck` 只有单个 BROKEN item，**没有部分释放后回滚**的用例；也没有“可重试失败不写 ack”的用例。代码路径存在，测试覆盖不足。
- **NB-6**：FAILED ack 本身写入失败时 `consumer.Permanent` + 原消息 DLQ，Order 收不到 FAILED；best effort，需 ops 从 DLQ 处理。

---

## 5. HELD/CONFIRMED/RELEASED/无行/未知

- HELD → `releaseTx`（旧 `Release` 的 tx 版）。
- CONFIRMED → `cancelReservationTx(..., r.quantity)`；与旧 `TccCancelConfirm` 使用 **reservation quantity** 一致，不用 event quantity。
- RELEASED → 跳过；无行（NotFound）→ 跳过（旧 consumer 的幂等 no-op）。
- 未知 status → `apperr.Invalid` → permanent → FAILED ack + DLQ。
- **NB-3**：`applyLegacyReleaseItemTx` 把 `reservationByOrderSku` 的 NotFound 当幂等 no-op；这与旧语义一致，但与 scope 里“NotFound 属永久失败”的措辞不同（no-row 不应 FAILED ack）。应明确：无行=幂等成功，其他 NotFound 才 permanent。
- **NB-4**：新 legacy 路径的 CONFIRMED/无行分支没有专门测试；旧 `TestRefundReleaseConsumerRestocksConfirmedOnce`/`SkipsMissingReservation` 只覆盖旧路径。新路径 `cancelReservationTx` quantity 依赖代码 inspection。

---

## 6. 幂等/并发

- 重复消息：第二次 reservation 已 RELEASED（或无行），`appendRefundReleaseAckTx` 计数发现 ack 已存在→跳过；测试 `TestLegacy24hReleaseAckInSameTransaction` 跑两次，ackCount=1。
- 并发：release 由 reservation 行锁/状态 CAS 串行化，重复释放不会发生；但 **ack 去重是 `SELECT COUNT(*)` + 无 `(event_type,aggregate_id)` 唯一键**（`outbox_events` 只有 `uk_outbox_event_id`）。
  - 同一 release_id 的并发消息在 reservation 行锁下通常第二笔会看到 RELEASED/ack；但**无行（no-row）分支没有行锁**，两个并发消息可能都 count=0 并各写一条 ack；`outbox_events` 无唯一键兜底。
  - **NB-5**：并发 no-row/同 release 的重复 ack 可能重复；建议加 `(event_type,aggregate_id)` 唯一键或按 release_id 的行锁/insert-ignore。
  - **NB-5b**：ack 去重不区分状态；已有 **FAILED** ack 会阻止后续修数据后的 **RELEASED** ack，Order 可能永远停在 FAILED。需要明确“FAILED 后重试是更新/覆盖还是新 release_id”。

---

## 7. 非 legacy / Phase 3 边界

- `handleRefundReleaseEvent` 只把 `source=legacy_24h` 路由到 `handleRefundReleaseLegacyAck`；其他 source 走旧 `handleRefundRelease`，旧测试全 PASS。
- 无 Order receipt、无 Payment、无 migration；`internal/services/order`/`migrations`/`api` 零改动；`git diff` = 0。
- **NB-7**：Phase 1 的 `TestRefundReleaseLegacy24hAcceptedByConsumer` 仍直接调旧 `handleRefundRelease`，不经过新 router；它只证明“旧路径也接受 legacy_24h”，未覆盖新 ack 路由。建议改走 `handleRefundReleaseEvent`。

---

## 8. AC-63

- Phase 2 只做 Inventory release + ack；Order receipt、Payment-last、`EXPIRED_REFUND` 全链路未做。
- AC-63 验收表保持 **未验证**；不升级。
- Phase 3/T5.2/3/5/6/7 未做。

---

## 9. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 同事务测试无故障注入（ack 失败→release 回滚） | `TestLegacy24hReleaseAckInSameTransaction` |
| NB-2 | 永久失败测试只有单 item，无部分释放回滚/可重试不写 ack | `TestLegacy24hReleasePermanentFailureWritesFailedAck` |
| NB-3 | no-row NotFound 当幂等成功，与 scope“NotFound 永久”措辞不同；无行不应 FAILED ack | `applyLegacyReleaseItemTx` |
| NB-4 | 新 legacy 路径 CONFIRMED/无行分支无专门测试（quantity 依赖 inspection） | 新测试覆盖 |
| NB-5 | ack 去重是 count，无 `(event_type,aggregate_id)` 唯一键；并发 no-row 可能重复 ack | `outbox_events` schema；`appendRefundReleaseAckTx` |
| NB-5b | FAILED ack 会阻止后续 RELEASED ack；重试语义需明确 | `appendRefundReleaseAckTx` 状态无关计数 |
| NB-6 | FAILED ack 写入失败 → DLQ，Order 收不到 FAILED | `handleRefundReleaseLegacyAck` |
| NB-7 | Phase 1 consumer 测试仍旧 router；未覆盖新 ack 路径 | `TestRefundReleaseLegacy24hAcceptedByConsumer` |

---

## 10. 裁决

**PASS（Phase 2）。** release + ack 同事务；永久失败回滚后写 FAILED ack（带 error）；可重试失败不写 ack；HELD/CONFIRMED/RELEASED/无行/未知分支与旧语义一致（CONFIRMED 用 reservation quantity）；非 legacy 走旧路径；无 Order/Payment/migration；测试与边界通过。

**Phase 2 可合入；Phase 3 Order receipt/Payment 未做；AC-63 仍未验证；NOT RELEASE。**

---

## 11. 复现命令（只读）

```bash
git rev-parse HEAD   # 9853e01...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/inventory -run 'TestRefundRelease|TestLegacy24hRelease' -count=1 -v
go test ./internal/platform/events -run 'TestRefundRelease' -count=1
go vet ./internal/services/inventory ./internal/platform/events
git diff 9853e01 -- internal/services/order migrations api | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
