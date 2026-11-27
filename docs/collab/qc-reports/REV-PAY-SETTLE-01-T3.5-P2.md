# REV-PAY-SETTLE-01-T3.5-P2 — T3.4/T3.5 Phase 2（Order 侧 responsibility/reason + quantity=0 + UNDETERMINED）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途；SPEC-0002 AC-60/AC-63 范围内） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/platform/refundpolicy/policy.go`（新共享枚举）；`internal/services/aftersales/responsibility.go`（枚举源 refactor）；`refund_composition.go` 的 responsibility/reason、UNDETERMINED 运费、quantity=0；`t3_5_phase2_integration_test.go`、更新后的 `t3_5_line_composition_integration_test.go`；plan。**不审** T3.6 AfterSales 接线、168h worker、post-departure partial 强制 lines。 |
| base | **`5db0f0b`**（`git rev-parse HEAD` = `5db0f0b9d6dae2e8cf5d7bac2bd7c4a42daccb6b`，Phase 1 已提交） |
| head | **工作树 @ `5db0f0b`（未提交）**：`refund_composition.go` `8aa0d0ef…`；`aftersales/responsibility.go` `87237f8d…`；plan `4758182f…`；settled test `7110890c…`；新增 `refundpolicy/policy.go` `61106c25…` + `policy_test.go` `31dbba21…`、`t3_5_phase2_integration_test.go` `722a1f75…`；`refund_admission.go` **未改**；`tasks.md` T3.4/T3.5 仍 `[ ]`。 |
| spec_version | **SPEC-0002 v1.11**（approved 2026-11-26；AC-60/AC-63） |
| adr_revision | **ADR-0007 Rev 2.3 / Amendment 1 §4.4**（行层 INV-24、组层、Payment 三层；INV-47） |
| baseline_impact | **有**：Order 对 `responsibility/reason_code` 做封闭枚举/配对校验；`UNDETERMINED` 禁运费；`quantity=0` 表示部分金额不消耗整件；AfterSales 的枚举常量/校验改为共享源（行为等价）。无 proto/migration；T3.6/168h 未做。 |
| INV | **INV-24**（行剩余/无超退）、**INV-47**（组成/责任/运费）、AC-60/63（仍不升级）、宪法 20/22/23。 |
| verification | **命令实测**：`go test ./internal/services/order -run 'TestAdmitRefund\|TestTccAdmission\|TestLegacy' -count=1` → **ok 42.730s**；`go test ./internal/platform/refundpolicy` → **ok 0.063s**；AfterSales 定向 `TestAC14FreightRefundSixScenarios\|TestAC33FreeTextDoesNotDetermineCarrier\|TestAC30ShortageCreatesSystemRefundAndPays` → **ok 1.302s**；`go vet ./internal/services/order ./internal/services/aftersales ./internal/platform/refundpolicy` → 0；AfterSales `-run '^$'` 编译通过；`git diff 5db0f0b -- migrations api docs/specs` = 0；`refund_admission.go` 未改。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **SCOPED-PASS（Phase 2 Order 侧）**：responsibility/reason 封闭枚举、UNDETERMINED 运费=0、quantity=0 通过；AfterSales 枚举源重构等价；**T3.6 AfterSales 接线与 168h 未做**；**AC-60/63 仍未验证**。0 阻断；NB-1…NB-5。 |

---

## 0. 结论

1. **Order 侧 responsibility/reason 生效**：`refundpolicy` 封闭枚举（5 party + 每 party reason 集合）；`validateRefundResponsibility` 空 responsibility 兼容旧调用、非空必须合法且 reason 配对、reason 无 responsibility 拒绝；非法 party/跨 party reason/orphan reason 测试全 PASS 且零行。
2. **UNDETERMINED 运费规则**：`responsibility == UNDETERMINED && freight > 0` 拒绝；`freight=0` 允许；商品仍可退；168h hold 未实现（AfterSales/T3.6），符合 INV-47/AC-64 “Order 只校验”。
3. **quantity=0**：允许部分金额不消耗整件；`quantity > 0` 仍校验数量剩余；`goods` 始终受行剩余约束；SETTLED 测试的 OK 请求改 quantity=0 并断言落库 0，PASS。
4. **AfterSales 重构等价**：5 个 party 常量值不变；reason 集合逐项相同；`validParty/validReason` 改为共享包；AfterSales 内无残留 `reasonCodes` 引用；编译 + 定向测试 PASS。
5. **兼容/TCC/zero**：空 responsibility 的旧 amount-only 路径不变；TCC/既有 `TestAdmitRefund` 全 PASS；`refund_admission.go` 未改。
6. **未做**：T3.6 AfterSales 责任接线、168h 运费挂起、post-departure partial 强制 lines；T3.4/T3.5 仍 `[ ]`；AC-60/63 未验证。

---

## 1. 审查输入与方法

- 读：`policy.go`/`policy_test.go`、`refund_composition.go` diff、`aftersales/responsibility.go` diff、Phase 2 测试、settled 测试 diff、plan diff、`refund_admission.go` 状态。
- 跑：order 回归（真实 MySQL）、refundpolicy 测试、AfterSales 定向测试、AfterSales 编译、vet、边界 diff。
- 未改代码/测试/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy' -count=1
ok   gin-kratos-gorm/internal/services/order  42.730s

$ go test ./internal/platform/refundpolicy
ok   gin-kratos-gorm/internal/platform/refundpolicy  0.063s

$ go test ./internal/services/aftersales -run 'TestAC14...|TestAC33...|TestAC30...' -count=1
ok   gin-kratos-gorm/internal/services/aftersales  1.302s

$ go vet ./internal/services/order ./internal/services/aftersales ./internal/platform/refundpolicy
exit 0
```

Phase 2 新测试 `TestAdmitRefundResponsibilityValidation` 覆盖：非法 party（`MERCHANT`）→ 拒绝+零行；跨 party reason（`MERCHANT_WAREHOUSE`+`LOST_IN_TRANSIT`）→ 拒绝+零行；orphan reason（空 responsibility+`OUT_OF_STOCK`）→ 拒绝+零行；`UNDETERMINED`+freight 200 → 拒绝+零行；`UNDETERMINED`+freight 0 → 通过；`MERCHANT_WAREHOUSE`+`OUT_OF_STOCK`+freight 200 → 通过且 header freight=200。settled 测试的 OK 请求改 quantity=0 并断言 commitment line quantity=0。

---

## 3. AfterSales 枚举 refactor 等价性

- **常量值**：`partyMerchant..partyUndetermined` 改为 `refundpolicy.Responsibility*` 别名，字符串值与旧代码完全相同（`MERCHANT_WAREHOUSE/CARRIER/CUSTOMER/PLATFORM/UNDETERMINED`）。
- **reason 集合**：旧 `responsibility.go` 的 `reasonCodes` 与新 `policy.go` 的 `reasonCodes` 逐项相同：Merchant 5 个、Carrier 3 个、Customer 3 个、Platform 2 个、Undetermined 1 个（`PENDING_REVIEW`）。
- **校验路径**：AfterSales `validParty/validReason` 改为调用共享包；旧调用点 `DetermineResponsibility` 不变；错误路径不变（仍是 `Invalid("responsibility/reason code invalid")` 之类）。
- **无遗漏引用**：AfterSales 内 `grep reasonCodes|validParty|validReason` 只命中 `responsibility.go` 的定义/调用；无其他文件依赖被删的 `reasonCodes`；AfterSales 包编译通过，定向 `TestAC14/AC30/AC33` PASS。
- **NB**：AfterSales 的 `validParty/validReason` **不**调用 `refundpolicy.Normalize`；旧实现也是大小写敏感，因此行为等价；但共享包提供了 Normalize，Order 用了而 AfterSales 没用，属一致性 nit（NB-3，不阻断）。

---

## 4. 空 responsibility 兼容 / TCC / zero

- `validateRefundResponsibility` 对空 responsibility + 空 reason 返回 `("", nil)`，旧 amount-only / 无 lines 调用不变。
- 空 responsibility + 非空 reason → `reason_code requires responsibility`，测试覆盖。
- 非空 responsibility 必须合法且 reason 配对；`UNDETERMINED` 只接受 `PENDING_REVIEW`。
- TCC 旧 amount-only 请求无 responsibility，`TestTccAdmission` PASS；zero 短路/既有 `TestAdmitRefund` PASS；`refund_admission.go` 未改（composition 调用仍是 Phase 1 的同一行）。
- **结论**：T3.6 之前的旧调用继续工作；无回归。

---

## 5. UNDETERMINED 运费（INV-47/AC-64）

- `UNDETERMINED` + `freight_refundable_cent > 0` → `Failed("UNDETERMINED responsibility must hold freight; freight_refundable_cent must be 0")`。
- `UNDETERMINED` + `freight=0` → 允许；商品照退。
- 运费 168h hold/AfterSales worker **不在本 Phase**；Order 只拒绝“责任未定却释放运费”，不实现挂起；符合 INV-47/AC-64 的“Order 只校验，不推断/不实现 T3.6”。
- 新测试覆盖 200 拒绝、0 通过。
- **NB**：无 lines 且 `UNDETERMINED` + freight>0 会先命中“freight requires an explicit line composition”，不是 UNDETERMINED 专用错误；两者都拒绝，不影响 invariant（NB-4，文档 nit）。

---

## 6. quantity=0 与数量超退

- `validateRefundComposition` 的请求行：`quantity < 0` 拒绝；`quantity == 0` 允许；`goods < 0` 拒绝。
- 行剩余：`goods > remainingGoods` 始终拒绝；`quantity > 0` 时才 `quantity > remainingQuantity` 拒绝；`quantity=0` 不消耗数量。
- 落库：请求 quantity=0 → commitment line quantity=0；settled 测试断言。
- `quantity>0` 时仍校验数量剩余；行超限测试的 4000/6000 请求 quantity=1，goods 超限 → “remaining line amount”。
- **NB**：没有“prior quantity 已用完、请求 quantity>0、goods 不超”的专门测试（代码有 `quantity > remainingQuantity` 检查，但测试只覆盖 goods 超限/quantity=0）；NB-1。
- **NB**：quantity=0 的“部分金额不消耗整件”语义（同一行可多次 quantity=0 直到 goods 用尽；之后 quantity>0 但 goods=0）是产品/架构确认的 NB-7 语义；Phase 2 按该语义实现，但建议产品/架构书面确认边界（NB-2，不阻断 Phase 2）。

---

## 7. 边界与越权

- `git diff 5db0f0b -- migrations api docs/specs` = **0**；无 proto/migration/契约改动。
- `refund_admission.go` **未改**（status 无该文件）；composition/commitment 插入仍是 Phase 1 的代码；Phase 2 只改 `refund_composition.go` + AfterSales 枚举源 + 测试 + plan。
- `tasks.md` T3.4/T3.5 仍 `[ ]`；plan 状态写“Phase 1+Phase 2 已实现；168h/T3.6 未做”，没有宣称 T3.4/T3.5 完成。
- 新增 `internal/platform/refundpolicy/`（policy.go + policy_test.go）；AfterSales 只换枚举源，无行为变化。
- 无生成物/二进制残留。

---

## 8. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 缺“prior quantity 已用完、请求 quantity>0、goods 不超”的数量超退测试；代码有 `quantity > remainingQuantity` 检查但未被测试覆盖 | `refund_composition.go` 行剩余；Phase 2 测试 |
| NB-2 | quantity=0 的“部分金额不消耗整件”边界（多次 0、之后 quantity>0 但 goods=0）建议产品/架构书面确认；Phase 2 已按 NB-7 实现，不阻断 | `refund_composition.go`；settled 测试 |
| NB-3 | AfterSales `validParty/validReason` 不调用 `refundpolicy.Normalize`，Order 用了；旧实现大小写敏感，行为等价，但共享包一致性 nit | `aftersales/responsibility.go`；`policy.go` |
| NB-4 | 无 lines 且 `UNDETERMINED`+freight>0 先命中“freight requires lines”，不是 UNDETERMINED 专用错误；两者都拒绝 | `refund_composition.go` 空 lines 分支 |
| NB-5 | Phase 2 测试的非法 party/跨 party/orphan 只断言 `err != nil`，未断言错误码；零行已断言 | `t3_5_phase2_integration_test.go` |

---

## 9. 裁决

**SCOPED-PASS（Phase 2 Order 侧）。** responsibility/reason 封闭枚举、UNDETERMINED 运费=0、quantity=0 已实现并通过真实 MySQL 回归；AfterSales 枚举源重构等价；空 responsibility/TCC/zero 兼容；`refund_admission.go` 未改；T3.4/T3.5 仍 `[ ]`。

**Phase 2 Order 侧可交人类合入；T3.6 AfterSales 接线与 168h 未做；AC-60/63 仍未验证；NOT RELEASE。**

---

## 10. 复现命令（只读）

```bash
git rev-parse HEAD   # 5db0f0b...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy' -count=1
go test ./internal/platform/refundpolicy
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/aftersales -run 'TestAC14FreightRefundSixScenarios|TestAC33FreeTextDoesNotDetermineCarrier|TestAC30ShortageCreatesSystemRefundAndPays' -count=1
go vet ./internal/services/order ./internal/services/aftersales ./internal/platform/refundpolicy
git diff 5db0f0b -- migrations api docs/specs | wc -l        # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 SCOPED-PASS 裁决。未改代码/测试/契约/迁移；未 commit、未 merge。
