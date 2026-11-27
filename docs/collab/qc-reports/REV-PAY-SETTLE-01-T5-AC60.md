# REV-PAY-SETTLE-01-T5-AC60 — AC-60 收口审查（无售后自动分配 + 发货 HELD/CONFIRMED/MISSING）

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `api/proto/mall/v1/mall.proto`（additive `client_request_id=11`）+ `api/gen/mall/v1/mall.pb.go`；`migrations/order/0025_legacy_refund_allocations.sql`；`internal/services/order/legacy_refund_allocation.go`；`refund_admission.go`/`service.go`（allocation/shipping）；T5 测试。**不审** T5.2/3/5/6/7。 |
| base | **`4202747`** |
| head | **工作树 @ `4202747`（未提交）**：proto/gen、0025、allocation、shipping、tests；`internal/services/inventory` 无改动。 |
| spec_version | **SPEC-0002 v1.11**（AC-60/AC-63/INV-46） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1**；**ADR-0011** |
| baseline_impact | **有**：proto additive；legacy 无售后自动分配；HELD→Confirm；MISSING/确认失败 fail-closed；无 Inventory 改动。 |
| INV | **INV-46/AC-60**（历史单读取/回填/退款/发货）、AC-63（保持已验）。 |
| verification | **命令实测**：`TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestLegacy\|TestT5\|TestAC60\|TestAC63\|TestRefundReleaseAck\|TestReleaseCommitment' -count=1` → **PASS（41.902s）**；2 个 AC-60 新测试单独 PASS（3.169s）；`make proto-guard` → **OK（1002 baseline）**；`go vet` 0；`git diff 4202747 -- internal/services/inventory` = **0**；gRPC stub diff = 0。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（AC-60 收口）**：proto additive/proto-guard OK；0025 分配表；无售后自动分配幂等；HELD→Confirm、MISSING/确认失败 fail-closed、CONFIRMED 代码路径；**AC-60 已验证；AC-63 保持已验证；T5.2/3/5/6/7 未做**。0 阻断；NB-1…NB-6。 |

---

## 0. 结论

1. **proto additive**：`AdmitRefundRequest.client_request_id = 11`；`mall.pb.go` 只新增字段/getter/raw descriptor；`mall_grpc.pb.go` 无 diff；`make proto-guard` OK（1002 baseline）。
2. **迁移 0025 additive**：`legacy_refund_allocations(order_no, request_id) PK`、`refund_id UNIQUE`、`created_at`；无破坏性 DDL；T5 真实迁移 24 个文件。
3. **分配幂等**：`refund_id` 为空时用 `(order_no, client_request_id)` 查映射；无则 `nextLegacyRefundSeq` 分配 `legacy:<order_no>:<seq>` 并插入映射；同 request_id 重试返回同一 refund_id；空 request_id 拒绝；并发 loser 消耗 seq、duplicate 后读 winner；seq gap 可接受。
4. **发货**：`checkOrderReady` 有 lines 时 `CONFIRMED` 放行、`HELD` 调 `Confirm`、`MISSING/其他` 返回 `inventory cannot be confirmed; shortage/refund required`；确认失败返回 `inventory confirmation failed; shortage/refund required`；不消费 permit；无 lines 的旧回复保持旧 `Ready` 行为。
5. **AC-60**：读取/回填（T5）、有售后、无售后自动分配、发货 HELD/MISSING 有测试；CONFIRMED 为代码路径（无专门新测试）；**AC-60 可标已验证**（带 NB）。**AC-63 保持已验证**；T5.2/3/5/6/7 未做。
6. **边界**：`internal/services/inventory` 0；无其他契约越界；proto 只 additive。

---

## 1. 审查输入与方法

- 读：proto/gen、0025、allocation、`AdmitRefund`/`checkOrderReady` diff、T5 测试。
- 跑：全量 order 回归、2 个 AC-60 新测试、proto-guard、vet、边界。
- 未改代码/契约/迁移；Spec AC-60 验收表按 PASS 更新；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestLegacy|TestT5|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment' -count=1
ok   ...order  41.902s

$ go test ... -run 'TestAC60LegacyRefundAllocationIdempotent|TestAC60ShippingConfirmsHeldAndBlocksMissing' -v
--- PASS: TestAC60LegacyRefundAllocationIdempotent
--- PASS: TestAC60ShippingConfirmsHeldAndBlocksMissing
    --- PASS: HELD_is_confirmed_before_the_permit
    --- PASS: MISSING_fails_closed_and_keeps_the_permit
PASS  (3.169s)

$ make proto-guard
[proto-guard] OK (1002 baseline entries)
```

---

## 3. proto / generated / 0025

- proto 只加 `client_request_id=11`；`mall.pb.go` +字段/getter/raw desc；`mall_grpc.pb.go` **无 diff**；`proto-guard` OK。
- 0025 additive：`order_no+request_id` PK、`refund_id UNIQUE`；`legacy_refund_allocations` 与 `legacy_refund_sequences` 分离；`T5` 真实迁移 24 个文件。
- `orderTestDB`/T5 迁移列表加入 0025；drop 列表加入 `legacy_refund_allocations`；真实迁移测试断言 `request_id` 列。

---

## 4. 分配幂等/并发

- `allocateLegacyRefundID`：trim；order/request_id 空 → Invalid；先查 `(order_no,request_id)`；命中返回；未命中 `nextLegacyRefundSeq`（独立 tx）→ `legacy:<order_no>:<seq>` → `INSERT` 映射；duplicate → 读 winner。
- 同 request_id 重试：返回同一 refund_id；测试断言 1 admission/1 allocation。
- 空 request_id：拒绝（`client_request_id is required when refund_id is empty`）。
- 并发 loser：`legacy_refund_allocations` PK + `refund_id UNIQUE`；loser 消耗 seq 后 duplicate，读 winner；seq gap 可接受。
- **NB-1**：无并发 loser 测试；`isDuplicateKey` 依赖 `errors.As`，代码路径存在但未实测。
- **NB-2**：无空 request_id/空 refund_id 的反例测试。
- **NB-3**：`AdmitRefund` 在 order 状态检查之前分配；不可退款订单会消耗 seq/写映射；可接受但需注意。
- 旧路径（refund_id 非空）不变；TCC/zero 旧测试 PASS。

---

## 5. 发货 HELD/CONFIRMED/MISSING

- `checkOrderReady`：
  - 无 lines → 旧 `!Ready` 行为（兼容旧 Inventory 回复）。
  - 有 lines：`CONFIRMED` → continue；`HELD` → `Confirm(ReserveRequest{...})`；`MISSING/其他` → `inventory cannot be confirmed; shortage/refund required`；`Confirm` 失败 → `inventory confirmation failed; shortage/refund required`。
- `AuthorizeShipment` 在错误时不消费 permit；测试 MISSING 后 permit 仍 `NONE`；HELD 后 Confirm 1 次、permit CONSUMED。
- **NB-4**：无 CONFIRMED 分支、Confirm 失败分支的专门测试；CONFIRMED 为代码路径。
- **NB-5**：D-11 确认失败上限未实现重试计数；失败返回可操作错误，ops 可 `ReportGroupShortage`，不构成永久阻断；建议后续补上限/告警。
- **NB-6**：无 lines 的旧回复兼容靠 `len(reply.GetLines())==0`；无专门回归测试，但旧 AC-60 发货测试使用 `t16InventoryPort`（无 lines）仍 PASS。

---

## 6. AC-60 / AC-63

- **AC-60 可标已验证**：读取/回填（T5 `TestT5RealMigration...`/`TestAC60Historical...`）、有售后、无售后自动分配（`TestAC60LegacyRefundAllocationIdempotent`）、发货 HELD/MISSING（`TestAC60Shipping...`）；CONFIRMED 为代码路径（NB-4）；D-11 上限为 NB-5；不阻断 AC-60。
- **AC-63 保持已验证**：Phase 3b 单组全链路 + 多组人工 ADR-0011 A；本 AC-60 改动未回退。
- T5.2/3/5/6/7 未做；T5.4 已 `[x]`。

---

## 7. 边界

- `git diff 4202747 -- internal/services/inventory` = **0**；无 Inventory 改动。
- proto additive；generated 仅 `mall.pb.go`，gRPC stub 无 diff；`make proto-guard` OK。
- 0025 additive；无其他契约越界。

---

## 8. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 无并发 loser 测试；`isDuplicateKey`/winner 读未实测 | `allocateLegacyRefundID` |
| NB-2 | 无空 request_id/空 refund_id 反例测试 | AC-60 新测试 |
| NB-3 | 分配在 order 状态检查前；不可退款订单消耗 seq/映射 | `AdmitRefund` |
| NB-4 | 无 CONFIRMED/Confirm 失败分支测试 | `checkOrderReady` |
| NB-5 | D-11 确认失败上限未实现重试计数；ops 可 `ReportGroupShortage`，非永久阻断 | `checkOrderReady` |
| NB-6 | 无 lines 旧回复兼容无专门回归测试（旧发货测试覆盖） | `checkOrderReady` |

---

## 9. 裁决

**PASS（AC-60 收口）。** proto additive/proto-guard OK；0025 分配表；无售后自动分配幂等；HELD→Confirm、MISSING/确认失败 fail-closed、CONFIRMED 代码路径；无 Inventory 改动；全量测试 PASS。

**AC-60 已验证；AC-63 保持已验证；T5.2/3/5/6/7 未做；NOT RELEASE。**

AC 位移：`docs/specs/0002` **AC-60 未验证 → 已验证**（`TestAC60LegacyRefundAllocationIdempotent`/`TestAC60ShippingConfirmsHeldAndBlocksMissing` + T5）；AC-63 保持已验证。

---

## 10. 复现命令（只读）

```bash
git rev-parse HEAD   # 4202747...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestLegacy|TestT5|TestAC60|TestAC63|TestRefundReleaseAck|TestReleaseCommitment' -count=1
make proto-guard
go vet ./internal/services/order ./cmd/order
git diff 4202747 -- internal/services/inventory | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决 + `docs/specs/0002` AC-60 验收状态。未改代码/契约/迁移；未 commit、未 merge。
