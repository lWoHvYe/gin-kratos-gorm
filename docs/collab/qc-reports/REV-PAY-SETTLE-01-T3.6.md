# REV-PAY-SETTLE-01-T3.6 — T3.6 AfterSales → Order 退款请求映射审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途；SPEC-0002 AC-60/AC-63 范围内） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/aftersales/workers.go` 的 `processRefundTask`/`refundAdmissionRequest`/`loadRefundLines`/`buildRefundAdmissionRequest`；新增 `refund_admission_request_test.go`；`t3.4-t3.5-plan.md` 的 T3.6 接线。**排除** `.dsh/skills/**`、`.specify/**`（Spec Kit 1.4.3→1.4.5 自动升级，不属本次变更）。**不审** T3.6 的 168h hold 实现（既有 AfterSales worker）、T3.4/T3.5 其余、T3.2/T5。 |
| base | **`07291e6`**（`git rev-parse HEAD` = `07291e6eedddd59138410f25e7786065faeaecd3`） |
| head | **工作树 @ `07291e6`（未提交）**：`workers.go` sha256 `3ea335a4…`；新增 `refund_admission_request_test.go` `cc7260b6…`；plan `2bf321aa…`。`internal/services/order`、`api`、`migrations`、`docs/specs` **未改**；`.dsh/.specify` 自动升级文件已从范围排除。 |
| spec_version | **SPEC-0002 v1.11**（approved 2026-11-26；AC-60/AC-63） |
| adr_revision | **ADR-0007 Rev 2.3 / Amendment 1 §4.4**（行层 INV-24、INV-47、责任/运费） |
| baseline_impact | **有**：AfterSales `processRefundTask` 向 Order 传 `lines/responsibility/reason/freight`；Order 侧不变；无 proto/migration；168h hold 仍由既有 AfterSales worker 执行。 |
| INV | **INV-24**（行级/组级/captured）、**INV-47**（组成/责任/运费）、AC-60/63（仍不升级）、宪法 20/22/23。 |
| verification | **命令实测**：`go test ./internal/services/aftersales -run 'TestBuildRefundAdmissionRequest' -v` → 4 个纯测试 PASS（0.089s）；`TEST_MYSQL_DSN=... go test ./internal/services/aftersales -run 'TestAC14\|TestAC30\|TestAC33\|TestFreightResponsibilityDisabled' -count=1` → **ok 2.074s**；Order Phase 2 回归 `TestAdmitRefund\|TestTccAdmission\|TestLegacy` → **ok 52.293s**；`go vet ./internal/services/aftersales ./internal/services/order` → 0；`git diff 07291e6 -- internal/services/order api migrations docs/specs` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **SCOPED-PASS（T3.6 映射）**：AfterSales→Order 的 lines/responsibility/freight 映射可用，legacy/zero/shortage/TCC 未回归；**168h hold 仍由 AfterSales 既有 worker**；**AC-60/63 仍未验证**。0 阻断；NB-1…NB-6。 |

---

## 0. 结论

1. **T3.6 映射实现**：`processRefundTask` 通过 `refundAdmissionRequest` 读 `after_sale_items` 的 `line_no/quantity/pending_amount_cent`，忽略 0 金额行；`buildRefundAdmissionRequest` 计算 `freight = effective_amount − Σ goods`，传 `lines/responsibility/reason`；`UNDETERMINED + freight>0` 拒绝；legacy `pending` 全 0 → amount-only。
2. **`pending_amount_cent`**：是 AfterSales 每行的待退金额；`shortage_consumer`/`service` 也用它；作为 `goods_cent` 传入 Order 正确。
3. **UNDETERMINED**：AfterSales 的 168h hold 路径使 `effective_amount_cent` 不含被 hold 的运费；有 lines 的请求 freight 残差为 0，Order 侧 `UNDETERMINED+freight=0` 允许；helper 对“effective 含运费”的请求拒绝，和 hold 语义一致。
4. **legacy pending=0**：helper 返回无 lines + amount-only（freight=0），T3.6 之前的 AfterSales 继续工作；纯测试覆盖。
5. **zero/shortage/TCC**：`processZeroRefundTask` 未改；`TestAC30`（短缺）/`TestAC14`（运费）PASS；Order TCC 回归 PASS。
6. **T3.6 可标 `[x]`（映射范围）**：plan 已写 T3.6 接线，tasks 仍 `[ ]`（作者更新）；**T3.4/T3.5 仍 `[ ]`**；AC-60/63 未验证。

---

## 1. 审查输入与方法

- 读：`workers.go` diff（`processRefundTask`→`refundAdmissionRequest`、`loadRefundLines`、`buildRefundAdmissionRequest`）、`refund_admission_request_test.go`、`after_sale_items.pending_amount_cent` 的写入/读取、AfterSales 的 `freightRefundFor`/`after_sale_freight_holds`、plan T3.6 段。
- 跑：纯测试、AfterSales 定向、Order Phase 2 回归、vet、边界。
- **排除**：`.dsh/skills/**`、`.specify/**` 自动升级文件；未把它们计入 diff/评审。
- 未改代码/测试/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/services/aftersales -run 'TestBuildRefundAdmissionRequest' -v
--- PASS: TestBuildRefundAdmissionRequest
--- PASS: TestBuildRefundAdmissionRequestLegacyAmountOnly
--- PASS: TestBuildRefundAdmissionRequestRejectsOverAmount
--- PASS: TestBuildRefundAdmissionRequestHoldsUndeterminedFreight
PASS  (0.089s)

$ TEST_MYSQL_DSN=... go test ./internal/services/aftersales -run 'TestAC14|TestAC30|TestAC33|TestFreightResponsibilityDisabled' -count=1
ok   ...aftersales  2.074s

$ TEST_MYSQL_DSN=... go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy' -count=1
ok   ...order  52.293s

$ go vet ./internal/services/aftersales ./internal/services/order
exit 0
```

---

## 3. `pending_amount_cent` 与 freight 残差

- `after_sale_items.pending_amount_cent` 是该行的待退金额（`service.go:353` 写入；`shortage_consumer.go`/`refund_lines.go` 读取）；`loadRefundLines` 把它作为 `RefundLine.GoodsCent`，正确。
- 忽略 `goods<=0` 行后，`freight = amount − Σ goods`；若所有行 goods=0（**商品 0、运费>0**），`len(lines)==0` → 返回 amount-only，`FreightRefundableCent=0`，freight 被并进 `amount_cent`。Order 旧 amount-only 路径可接受，但 **freight 不会作为 line 0 传给 Order，责任/运费无法按行校验**。若新 AfterSales 可能出现“纯运费、无商品行”的退款，需要产品/架构确认；NB-1，不阻断纯映射（legacy 回退是故意的）。
- `loadRefundLines` 对 `goods <= 0` 全部跳过，**包括负数**；负数 `pending_amount_cent` 会被静默忽略，而不是 fail-closed。NB-2。

---

## 4. UNDETERMINED 与 168h hold

- AfterSales 的 `freightRefundFor` 对 `UNDETERMINED` 返回 `hold=true`，运费进入 `after_sale_freight_holds`（168h worker），`effective_amount_cent` 为不含该运费的金额。
- 映射时 `freight = effective − Σ goods`；若 effective 已排除 hold 运费，有 lines 的请求 freight=0，Order 的 `UNDETERMINED+freight=0` 允许。
- helper 对“effective 含运费”（freight>0）拒绝，纯测试 `TestBuildRefundAdmissionRequestHoldsUndeterminedFreight` 覆盖；与 168h hold 语义一致（Order 不自行释放）。
- **NB-4**：helper 用原始 `responsibility == "UNDETERMINED"` 判断，未用 `refundpolicy.Normalize`；Order 侧会 normalize，所以即便 DB 值为小写，Order 仍会拒绝；只是 helper 的预检可能不触发。
- **NB-5**：无 lines 的早返回在 UNDETERMINED 检查之前；若 legacy 行 UNDETERMINED 且 effective 含运费，会走 amount-only 而不触发 helper 拒绝。当前 AfterSales 对 UNDETERMINED 的 effective 已排运费，属边界。

---

## 5. legacy `pending=0` 兼容

- `loadRefundLines` 返回空 → `buildRefundAdmissionRequest` 直接返回 `AmountCent=amount`、无 lines、freight=0；Order 的旧 amount-only 路径继续工作。
- 纯测试 `TestBuildRefundAdmissionRequestLegacyAmountOnly` 断言 lines=0/amount=5000/freight=0。
- **结论**：T3.6 之前的 AfterSales（无 line composition）不会被破坏。

---

## 6. zero / shortage / TCC

- `processZeroRefundTask` 未改；zero 路径仍走 Order 的零元短路。
- 定向 `TestAC30`（短缺系统退款）、`TestAC14`（运费六场景）、`TestAC33`（自由文本不决定承运商）、`TestFreightResponsibilityDisabled` 全 PASS。
- Order `TestTccAdmission` 回归 PASS；映射只改变 AfterSales 发给 Order 的请求字段，不改 TCC/zero 分支。
- **NB-6**：没有真实 `processRefundTask` 集成断言 `AdmitRefundRequest.lines/responsibility/freight`；现有纯测试 + 定向测试覆盖映射函数和既有路径，按用户要求记为 NB，不阻断。

---

## 7. 边界与任务

- `git diff 07291e6 -- internal/services/order api migrations docs/specs` = **0**；Order/api/migration/spec 未改。
- 本次只改 `internal/services/aftersales/workers.go` + 新增纯测试 + plan；`.dsh/.specify` 自动升级已排除。
- `tasks.md` T3.6 仍 `[ ]`，T3.4/T3.5 仍 `[ ]`；plan 已写“T3.6 AfterSales 映射已实现；待 reviewer”。
- **T3.6 可标 `[x]`（映射范围）**：映射 + 责任/运费 + legacy/zero/shortage 定向测试通过；168h hold 仍由既有 AfterSales worker；建议作者在台账 PASS 后把 T3.6 标 `[x]`，**T3.4/T3.5 保持 `[ ]`**。

---

## 8. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 所有行 goods=0（商品 0、运费>0）→ 返回 amount-only，freight 不单独传给 Order；若新 AfterSales 有“纯运费”退款，需产品/架构确认 | `loadRefundLines`；`buildRefundAdmissionRequest` 早返回 |
| NB-2 | `loadRefundLines` 对 `goods<=0`（含负数）一律跳过；负数 pending 不 fail-closed | `workers.go:694-710` |
| NB-3 | 无真实 `processRefundTask` 集成断言 `AdmitRefundRequest` 的 lines/responsibility/freight；纯测试+定向测试覆盖映射函数 | `refund_admission_request_test.go` |
| NB-4 | helper 的 `UNDETERMINED` 判断未用 `refundpolicy.Normalize`；Order 侧 normalize 后仍会拒绝 | `buildRefundAdmissionRequest` |
| NB-5 | 无 lines 早返回在 UNDETERMINED freight 检查之前；legacy UNDETERMINED+effective 含运费会走 amount-only | `buildRefundAdmissionRequest` |
| NB-6 | 纯测试未覆盖 zero/shortage/TCC 的映射；既有定向测试覆盖运行路径 | 测试范围 |

---

## 9. 裁决

**SCOPED-PASS（T3.6 映射）。** AfterSales→Order 的 lines/responsibility/freight 映射可用；legacy amount-only、zero、shortage、TCC 未回归；168h hold 仍由既有 AfterSales worker；`internal/services/order`/api/migrations/spec 未改；T3.6 可标 `[x]`（映射范围），T3.4/T3.5 保持 `[ ]`。

**T3.6 映射可交人类合入；168h hold 仍由 AfterSales 既有 worker；AC-60/63 仍未验证；NOT RELEASE。**

---

## 10. 复现命令（只读）

```bash
git rev-parse HEAD   # 07291e6...
git status --porcelain -- internal/services/aftersales internal/services/order docs/lead/PAY-SETTLE-01 docs/specs api migrations
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
go test ./internal/services/aftersales -run 'TestBuildRefundAdmissionRequest' -count=1
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/aftersales -run 'TestAC14|TestAC30|TestAC33|TestFreightResponsibilityDisabled' -count=1
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy' -count=1
go vet ./internal/services/aftersales ./internal/services/order
git diff 07291e6 -- internal/services/order api migrations docs/specs | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 SCOPED-PASS 裁决。未改代码/测试/契约/迁移；未 commit、未 merge。
