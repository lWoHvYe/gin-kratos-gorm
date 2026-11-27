# REV-PAY-SETTLE-01-T5-AC59B — T5.3 AC-59 第 2/3 分支 + AC-57 CREATED 审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/order/shortage.go`（`ChooseShortageResolution` 无 case）；`internal/services/fulfillment/delivery.go`（`confirmReceiptTx` 非 SHIPPED/RECEIVED）；新测试 `t5_ac59_shortage_test.go`、`delivery_integration_test.go`。**不审** AC-59 第 4 分支（库存失败先于支付成功）、AC-57 其余条款。 |
| base | **`54013a6`** |
| head | **工作树 @ `54013a6`（未提交）**：shortage/delivery + tests；无 proto/migration。工作树另有未跟踪 `docs/archive/2026-11-27-governance/{AGENTS_archive,constitution_archive}.md`，**非本次变更**。 |
| spec_version | **SPEC-0002 v1.11**（AC-59/AC-57；AC-60/63 不变） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1** |
| baseline_impact | **有**：乱序缺货解决/收货事件 fail-closed；只 order/fulfillment；无 proto/migration。 |
| INV | **AC-59**（乱序事件）、**AC-57**（收货必须先发货）；AC-60/63 不变。 |
| verification | **命令实测**：order `TestAC59\|TestShortage\|TestLegacy\|TestT5` → **PASS（40.648s）**；fulfillment `TestAC59\|TestAC2\|TestAC3\|TestDelivery` → **PASS（5.660s）**；`go vet ./internal/services/order ./internal/services/fulfillment` → 0；`git diff 54013a6 -- api migrations` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（AC-59 第 2/3 分支 + AC-57 CREATED）**：缺货无 case 返回 `EVENT_OUT_OF_ORDER`、零建案；receipt 非 SHIPPED/RECEIVED（含 CREATED）在写入前拒绝、RECEIVED 幂等保持。**AC-59 3/4；AC-57 部分（仅 CREATED）；AC-60/63 不变**。0 阻断；NB-1…NB-4。 |

---

## 0. 结论

1. **缺货无 case**：`ChooseShortageResolution` 的 `sql.ErrNoRows` 分支不再普通 NotFound，返回 `EVENT_OUT_OF_ORDER: ... shortage report` + `s.log.Warn`；不建案、不写 outbox；测试断言 `order_shortage_cases` 零行。
2. **receipt 乱序**：`confirmReceiptTx` 在读取 shipment 后、任何 evidence/status/receipt 写入前；`status != SHIPPED && status != RECEIVED`（含 `CREATED`）→ `EVENT_OUT_OF_ORDER: receipt arrived before shipment`；`CANCELLED` 仍旧错误；`RECEIVED` 幂等路径保持；测试 CREATED 后状态不变、零 receipt。
3. **AC-57**：本轮只覆盖 `CREATED` 拒绝；AC-57 其余条款（收货后状态/证据/幂等细节）未审；**AC-57 为“部分”，不能标已验证**。
4. **AC-59 3/4**：第 1（退款投影先于捕获）、第 2（收货先于发货）、第 3（缺货解决先于上报）已做；第 4（库存失败先于支付成功）未做；AC-59 不能标已验证；T5.3 仍部分。
5. **AC-60/63 不变**：AC-60/63 保持已验证，无回退。
6. **边界**：无 proto/migration；只改 order/fulfillment；archive 文件非本次变更。

---

## 1. 审查输入与方法

- 读：shortage/delivery diff、新测试、`confirmReceiptTx` 状态机、`TestAC20`（SHIPPED）与 `RECEIVED` 路径。
- 跑：order/fulfillment 测试、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestAC59|TestShortage|TestLegacy|TestT5'
ok   ...order  40.648s

$ TEST_MYSQL_DSN=... go test ./internal/services/fulfillment -run 'TestAC59|TestAC2|TestAC3|TestDelivery'
ok   ...fulfillment  5.660s

$ go vet ./internal/services/order ./internal/services/fulfillment   # 0
$ git diff 54013a6 -- api migrations | wc -l   # 0
```

---

## 3. 缺货无 case

- `ChooseShortageResolution`：`SELECT` 无行 → `s.log.Warn` + `apperr.Failed("EVENT_OUT_OF_ORDER: shortage resolution arrived before the shortage report")`；不建案/不写 outbox。
- 测试：`ORD-AC59-SHORT` 无 case；错误含 `EVENT_OUT_OF_ORDER`；`order_shortage_cases` 零行。
- **NB-1**：告警只有 `s.log.Warn`，无指标/告警规则；ops 可观测性建议补，不阻断。
- **NB-2**：无 case 的调用方重试/终止语义未测试；`EVENT_OUT_OF_ORDER` 可识别。

---

## 4. receipt 非 SHIPPED/RECEIVED

- `confirmReceiptTx`：`CANCELLED` → 旧错误；`status != SHIPPED && status != RECEIVED` → `EVENT_OUT_OF_ORDER: receipt arrived before shipment`；在 `delivery_disputes`/evidence/status/receipt 写入之前返回。
- `SHIPPED`/`RECEIVED` 继续原路径；`RECEIVED` 幂等保持；`TestAC20DuplicateDelivered` 的 shipment 是 `SHIPPED`，不受影响。
- 测试 `CREATED`：错误含 `EVENT_OUT_OF_ORDER`、状态仍 `CREATED`、`shipment_receipts` 零行。
- **NB-3**：未来若出现 `DELIVERED` 等新 shipment 状态，会被当前守卫判乱序；需随状态机扩展。当前 shipment 状态集为 CREATED/SHIPPED/RECEIVED/CANCELLED，`DELIVERED` 是 tracking 事件而非 shipment 状态。
- **NB-4**：无 `CANCELLED`/`RECEIVED` 幂等的新专门测试，但既有 AC-2/3/20 路径覆盖。

---

## 5. AC-57 / AC-59 / AC-60/63

- **AC-57 部分**：只覆盖 `CREATED` 拒绝；其余条款未审；不能标已验证。
- **AC-59 3/4**：第 1/2/3 分支已做；第 4（库存失败先于支付成功）未做；AC-59 不能标已验证；T5.3 仍部分。
- **AC-60/63 不变**：AC-60 已验证、AC-63 已验证，本轮无回退。

---

## 6. 边界

- `git diff 54013a6 -- api migrations` = **0**；无 proto/migration。
- 只改 `internal/services/order/shortage.go`、`internal/services/fulfillment/delivery.go` + 测试；无其他契约越界。
- 工作树未跟踪 `docs/archive/2026-11-27-governance/{AGENTS_archive,constitution_archive}.md` 不属本次变更，未触碰。

---

## 7. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 缺货无 case 告警只有 `s.log.Warn`，无指标/告警规则 | `ChooseShortageResolution` |
| NB-2 | 无 case 的调用方重试/终止语义未测试 | 新 shortage test |
| NB-3 | 未来新 shipment 状态（如 DELIVERED）会被当前 SHIPPED/RECEIVED 守卫判乱序 | `confirmReceiptTx` |
| NB-4 | 无 CANCELLED/RECEIVED 幂等的新专门测试（既有 AC-2/3/20 覆盖） | delivery tests |

---

## 8. 裁决

**PASS（AC-59 第 2/3 分支 + AC-57 CREATED）。** 缺货无 case 返回 `EVENT_OUT_OF_ORDER`、零建案；receipt 非 SHIPPED/RECEIVED（含 CREATED）在写入前拒绝、RECEIVED 幂等保持；测试/vet/边界通过。

**AC-59 3/4；AC-57 部分（仅 CREATED）；AC-60/63 不变；NOT RELEASE。**

---

## 9. 复现命令（只读）

```bash
git rev-parse HEAD   # 54013a6...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAC59|TestShortage|TestLegacy|TestT5' -count=1
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/fulfillment -run 'TestAC59|TestAC2|TestAC3|TestDelivery' -count=1
go vet ./internal/services/order ./internal/services/fulfillment
git diff 54013a6 -- api migrations | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
