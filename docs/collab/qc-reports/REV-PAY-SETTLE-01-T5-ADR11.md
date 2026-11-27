# REV-PAY-SETTLE-01-T5-ADR11 — ADR-0011 + T5 证据审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更**（Spec/代码混合：ADR 契约 + 真实迁移/AC60/63 测试） |
| 阶段 | **Spec/代码** |
| review_scope | `docs/adr/0011-legacy-24h-release-ack.md`；`t5-ac63-release-ack-plan.md`；`t5-status.md`；`t5_migration_integration_test.go`、`t5_ac60_ac63_integration_test.go`；`settlement_integration_test.go` 的迁移 runner；`tasks.md` T5.4。**不审** ack 实现（未做）、T5.2/3/5/6/7。 |
| base | **`a92c53a`** |
| head | **`0416bc6`**（`git rev-parse HEAD` = `0416bc6d8ac406837988ae7567dde5f7813e7bc3`；工作树干净） |
| spec_version | **SPEC-0002 v1.11**（AC-60/AC-63/INV-46） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1**；**ADR-0011 Accepted**（2026-11-27 人类裁决 A） |
| baseline_impact | **有**：新 source/reason `legacy_24h`/`LEGACY_24H_EXPIRY`、Inventory ack 事件、Order `refund_release_receipts`（additive）、Payment-last/失败 COMPENSATING；本轮无迁移/proto；T5 测试真实迁移。 |
| INV | **INV-46**（历史支付解析收敛）、**AC-63**（COMPENSATING→EXPIRED_REFUND）、**AC-60**（历史 PAID 读取/回填/退款/发货）；AC-60/63 仍不升级。 |
| verification | **命令实测**：`go test ./internal/services/order -run 'TestT5RealMigration\|TestAC60\|TestAC63' -count=1 -v` → **PASS（7.984s）**；`TestT5RealMigrationSchemaAndAC60Backfill` 2.27s（22 个真实迁移）；AC-60 2.48s；AC-63 3.08s；`go vet ./internal/services/order` → 0；`git diff a92c53a..0416bc6 -- migrations api` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（ADR-0011 + T5 证据）**：`legacy_24h`/ack/receipt/Payment-last/失败 COMPENSATING 与 AC-63/INV-46 一致；ack 屏障是 receipt/lease 而非仅事件；T5.4 真实迁移/AC-60 回填/AC-63 前缀 PASS；**ADR-0011 可进入 Phase 1 实现**；**AC-60/63 仍未验证**；T5.2/3/5/6/7 未做。0 阻断；NB-1…NB-6。 |

---

## 0. 结论

1. **ADR-0011 与 AC-63/INV-46 一致**：`legacy_24h` + `LEGACY_24H_EXPIRY` 是新增 source/reason，不复用 `tcc/system_zero/system_shortage/aftersale`，也不复用 `payment/legacy_expired`；release 只单组、多组/无组人工；ack 事件同事务；Order 回执；Payment 只在 `RELEASED` 后；`FAILED`/超时保持 `COMPENSATING`。
2. **ack 屏障可验证**：ADR #4 `refund_release_receipts`（PENDING/RELEASED/FAILED + lease）；#5 只有 `RELEASED` 才 `Payment.Refund`；#6 Payment 前必须读 receipt，不依赖 outbox append/consumer 日志/轮询 Inventory DB；不是只加事件。
3. **部署/回滚**：consumer-first（Inventory ack producer/consumer → Order ack consumer/receipt → Order legacy producer）；回滚不得先回退 Inventory consumer；旧 consumer 必须先升级。**NB-3**：旧 consumer DLQ 行为未显式写；consumer-first 是控制。
4. **T5 测试真实**：`t5FreshMigrationDB` 按序执行 `migrations/order/*.sql` **22 个真实文件**（非内联 DDL；含 0014/0020 触发器 `DELIMITER $$`）；AC-60 回填在真实 schema 上；AC-63 覆盖 unique/ambiguous/zero/COMPENSATING 前缀；**没有把前缀冒充全量**（AC-63 表仍 `未验证`）。
5. **T5.4 `[x]` 适当**：真实迁移 + AC-60 回填证据；T5.1/2/3/5/6/7 仍 `[ ]`。
6. **AC-60/63 仍未验证**：AC-63 全链路（ack/Payment/EXPIRED_REFUND）未实现；AC-60 的“无售后单自动分配/Inventory HELD/缺货”缺口仍在；不升级。

---

## 1. 审查输入与方法

- 读：ADR-0011、T5 plan/status、T5 测试、迁移 runner、`tasks.md`、AC-63/INV-46、`legacy-release`/`r0.4` addendum 的 DEFER/§2.3.2。
- 跑：T5 迁移/AC-60/AC-63 测试、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/services/order -run 'TestT5RealMigration|TestAC60|TestAC63' -count=1 -v
--- PASS: TestAC60HistoricalPaidReadBackfillRefundShip (2.48s)
--- PASS: TestAC63LegacyResolutionAndQueue (3.08s)
--- PASS: TestT5RealMigrationSchemaAndAC60Backfill (2.27s)
PASS  (7.984s)

$ go vet ./internal/services/order   # 0
$ git diff a92c53a..0416bc6 -- migrations api | wc -l   # 0
```

T5.4：`t5FreshMigrationDB` 建 disposable DB，按序执行 22 个 `migrations/order/*.sql`；断言 `orders.settlement_payment_no`、commitment goods/composition/lines、release、legacy queue/sequence 列；历史 PAID + 唯一捕获 → `BackfillLegacyPAID` 回填。

---

## 3. ADR-0011 与 AC-63/INV-46

- **source/reason**：#1 `legacy_24h`/`LEGACY_24H_EXPIRY`；**不复用** `tcc/system_zero/system_shortage/aftersale`，**不复用** `payment/legacy_expired`。与 AC-63/INV-46 的“不得猜测/不得按 0 额度/不得长期 OPEN”一致；新 source 不伪造业务来源。
- **release 请求**：#2 `release_id=rr:<refund_id>`、`refund_id=pay:<payment_no>:legacy_expired`、`release_scope=order_reservation_full`、`group_count=1`；仅单组；多组/无组人工。**NB-1**：ADR 的“多组/无组…保持 COMPENSATING/人工”未 carve out AC-63 的“有离仓组→EXPIRED_FINANCE”；应明确“有离仓组仍 EXPIRED_FINANCE，只有无离仓多组/无组人工 COMPENSATING”。
- **ack/receipt/Payment-last/失败**：#3 Inventory 同事务 outbox ack；#4 Order `refund_release_receipts`（PENDING/RELEASED/FAILED/lease）；#5 只有 RELEASED 才 Payment，成功 `EXPIRED_REFUND`，FAILED/超时 COMPENSATING；#6 屏障；完整。
- **NB-2**：ADR #1 “禁止 legacy_expired”与 #2 `refund_id=...:legacy_expired` 并存；应说明 `legacy_expired` 是既有 **refund_id** 后缀，不是新 source；新 source 是 `legacy_24h`。旧 `legacy-release` addendum §5 的 `legacy_expired` source 提案也应明确被 `legacy_24h` 取代（“取代”行目前只写 DEFER 结论）。
- **NB-6**：ADR 的“后果”写新表/迁移；本轮 `git diff` 无 migration/proto；`refund_release_receipts` 迁移属 Phase 1/3 实现，ADR 是契约批准，不是本 commit 的迁移文件。边界 0 成立。

---

## 4. ack 前不 Payment 的屏障

- #4 receipt 与 release 请求**同一事务**写 `PENDING`；ack consumer 按 `release_id` 幂等更新；#5 Payment 只在 `RELEASED` 后；#6 不依赖 outbox append 成功/consumer 日志/轮询 Inventory DB；CAS/lease 防并发。
- 这是**可验证的 DB 屏障**：`refund_release_receipts.state=RELEASED` 是 Payment 的前置条件；`FAILED`/超时零 Payment；重复 ack 幂等。不是“只加一个 source/事件”。
- **NB-4**：ADR“核对”列出的 `TestLegacy24hReleaseAck`/`TestLegacy24hReleaseSourceContract`/`TestAC63` 全链路尚未存在（Phase 2/4）；ADR 应把这些标为 Phase 2/4 planned，避免被当作现有证据。

---

## 5. 部署顺序/旧 consumer DLQ/回滚

- ADR #3：旧 consumer 先升级，Order producer 后才能发送；#7：Inventory ack producer/consumer → Order ack consumer/receipt → Order legacy producer；回滚不得先回退 Inventory consumer。
- plan Phase 1：registry/validator + consumer 契约测试；**不发送 producer 直到 consumer 就绪**；Phase 2：成功/DLQ/失败 ack、重复释放幂等、同事务。
- **NB-3**：ADR/plan 没有逐字写“旧 consumer 收到未知 event 的 DLQ/告警”处置；consumer-first + 旧 consumer 先升级是控制，但 DLQ 行为应写清（尤其回滚时旧 consumer 重新上线）。

---

## 6. T5 测试真实性

- **22 个真实迁移**：`t5FreshMigrationDB` 用 `filepath.Glob(migrations/order/*.sql)` + `applyOrderTestMigration`；`settlement_integration_test.go` 的 runner 支持 `DELIMITER $$`（0014/0020 触发器）；不是内联 DDL。
- **AC-60 回填**：真实 schema 上 `BackfillLegacyPAID` → `settlement_payment_no=PAY-T5-AC60`；`TestAC60HistoricalPaidReadBackfillRefundShip` 另测读取/回填/有售后退款/发货（inline `legacyTestDB`，行为级）。
- **AC-63 前缀**：`TestAC63LegacyResolutionAndQueue` 覆盖 unique→RESOLVED、ambiguous→OPEN、zero→EXPIRED_ZERO、captured unshipped→COMPENSATING（permit revoked、零 Payment）；**没有** ack/Payment/EXPIRED_REFUND，故不是全量；`tasks.md` T5.4 `[x]` 只对应“真实迁移 + AC-60 回填”。
- **没有把前缀冒充全量**：AC-63 验收表仍是 `未验证`；`t5-status.md` 明确“AC-63 不能标已验证”。

---

## 7. AC 状态 / T5.4

- **T5.4 `[x]` 适当**：真实迁移（22 文件）+ AC-60 回填；T5.1/2/3/5/6/7 仍 `[ ]`。
- **AC-60/63 仍 `未验证`**：AC-63 全链路（ack/Payment/EXPIRED_REFUND）未实现；AC-60 的“无售后单自动分配/Inventory HELD/缺货”缺口仍在（`t5-status.md` §3）；不升级。
- **AC-62** 已由 T3.2 验证（前轮），不在本轮范围。

---

## 8. 边界

- `git diff a92c53a..0416bc6 -- migrations api` = **0**；无新迁移/proto。
- 变更仅：ADR-0011、T5 plan/status、T5 测试、`settlement_integration_test.go` 迁移 runner、`tasks.md`；无生产代码/契约/迁移。
- **NB-5**：`t5-status.md` 写“基线 a92c53a + 本轮 T5 测试（未提交）”，但 0416bc6 已提交；状态文档的“未提交”过时。
- **NB-6**：ADR 的“新表/迁移”是 Phase 1/3 实现项；本 commit 无迁移；ADR 是契约批准，不是迁移文件。

---

## 9. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | ADR #2 多组/无组“COMPENSATING/人工”未 carve out AC-63“有离仓组→EXPIRED_FINANCE” | ADR-0011 §决策 #2；AC-63/INV-46 |
| NB-2 | ADR #1 禁止 `legacy_expired` 与 #2 `refund_id=...:legacy_expired` 并存；旧 `legacy-release` §5 的 `legacy_expired` source 提案未被“取代”行显式覆盖 | ADR-0011 #1/#2；“取代”行 |
| NB-3 | 旧 consumer 的 DLQ/告警行为未显式写；consumer-first/旧 consumer 先升级已写 | ADR #3/#7；plan Phase 1/2 |
| NB-4 | ADR“核对”列出 `TestLegacy24hReleaseAck`/`SourceContract`/`TestAC63` 全链路，当前不存在；应标 Phase 2/4 planned | ADR-0011 §核对 |
| NB-5 | `t5-status.md` 说测试“未提交”，但 0416bc6 已提交 | `t5-status.md` §5 |
| NB-6 | ADR“后果”写新表/迁移；本 commit 无 migration/proto；迁移属 Phase 1/3 | ADR-0011 §后果；`git diff` |

---

## 10. 裁决

**PASS（ADR-0011 + T5 证据）。** `legacy_24h`/ack/receipt/Payment-last/失败 COMPENSATING 与 AC-63/INV-46 一致；ack 是 receipt/lease 屏障；T5.4 真实迁移/AC-60 回填/AC-63 前缀 PASS；无迁移/proto/生产代码改动；T5.4 `[x]` 适当。

**ADR-0011 可进入 Phase 1 实现；AC-60/63 仍未验证；T5.2/3/5/6/7 未做；NOT RELEASE。**

---

## 11. 复现命令（只读）

```bash
git rev-parse HEAD   # 0416bc6...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestT5RealMigration|TestAC60|TestAC63' -count=1 -v
go vet ./internal/services/order
git diff a92c53a..0416bc6 -- migrations api | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
