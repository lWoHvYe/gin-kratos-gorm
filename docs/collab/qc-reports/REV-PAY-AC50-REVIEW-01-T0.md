# REV-PAY-AC50-REVIEW-01-T0 — T0+T0b+T0c 发布单元 scoped 复审

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01`（Parent `PAY-SETTLE-01` 冻结） |
| Tier | **L2（确认）**：跨 Fulfillment/Order RPC 响应语义、资金/退款准入、状态机；不适用 L1 |
| review_scope | **仅 T0（Fulfillment `<NO_WAYBILL>` + proto）、T0b（Order REVIEW 持久化/幂等/零元/legacy）、T0c（Tx2 双谓词 CAS）**；不审 T1–T8；不声称 AC-VERIFIED/Release |
| base | **`a34c845`**（C-5 errata + P5 批准） |
| code head | **`e374384107ca17c266054033c448eb33981445d7`**（7 files，+807/−24） |
| head（复审绑定） | **`c796580399e2107d91d5b6e1627204ca61acbdc9`**（selfcheck；代码与 e374384 一致） |
| 契约输入 | SPEC-0002 v1.11（AC-50/AC-69/AC-70）、ADR-0007 AC-50 Addendum **0.6 ACCEPTED**、impact 0.4、Plan/Tasks P5 后 |
| integration_owner | System Architect（契约）；Order Owner（T0b/T0c）；Fulfillment（T0）；ops（人工队列，T1/T2 后） |
| INV | **INV-45**（permit/commitment 保持）、**INV-51**（REVIEW 有界/不可自动收敛）、**INV-52**（partial 不产生取消副作用——T0 不覆盖，见 §5） |
| affected_AC | AC-50/AC-69/AC-70；AC-72 后端不在 T0；**不新增 AC-VERIFIED** |
| verification | §1：MySQL 8.4.13 独立 schema 重跑 `TestAC50Review`（plain/`-race`）、`TestCancelIfCreated`、`TestAdmitRefund`；`make proto-guard` OK；`migrations` diff 空 |
| 日期 / Actor / Reviewed-by | 2026-11-26 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（T0/T0b/T0c scoped）**；不可单独上生产（T1/T2/T3 未落地），见 §5 |

---

## 0. 结论

**PASS（有范围）。** 在 `HEAD c796580` / `code e374384` 上实际重跑：真实 MySQL 8.4.13、每包独立 schema；T0/T0b/T0c 的目标用例与 `-race` 全 PASS；`make proto-guard` OK；`migrations` diff 为空。代码满足 T0 的 F1/F1b/F2–F6/F8、T0b 的 O1/O7/O10、T0c 的 O11/O11b/O11c/O11d/W5/W13 目标；REVIEW 不被洗成 PRE，无 release/`Payment.Refund`，permit/commitment 保持。

**但这是 `T0` 单元 PASS，不是 AC-50/AC-72 全量、不是 Release PASS，也不能单独上生产**：T1（TCC `MANUAL_HOLD`）、T2（AfterSales 镜像/8 次）、T3（partial 三分类/INV-52）未落地；T0b 当前对任何 `CONSUMED+NO_WAYBILL`（含 partial）都会写 REVIEW，partial 的最终语义由 T3 决定。部署顺序必须 Fulfillment → Order（同一发布单元；若拆分，先 Fulfillment、再 T0c、最后 T0b；新 Order + 旧 Fulfillment 会把无行当 CANCELLED→PRE，禁止）。

---

## 1. 实际执行（HEAD c796580 / code e374384）

```bash
# MySQL 8.4.13，per-package schema
TEST_MYSQL_DSN=.../mall_integration_t0_order go test ./internal/services/order -run 'TestAC50Review' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t0_order_race go test -race ./internal/services/order -run 'TestAC50Review' -count=1
TEST_MYSQL_DSN=.../mall_integration_t0_ff go test ./internal/services/fulfillment -run 'TestCancelIfCreated' -count=1 -v
TEST_MYSQL_DSN=.../mall_integration_t0_order_existing go test ./internal/services/order -run 'TestAdmitRefund' -count=1
make proto-guard
git diff --stat a34c845..HEAD -- migrations
```

| 命令 | 结果 |
|---|---|
| order `TestAC50Review` | **PASS 11.965s**（8 个用例：Mapping/Zero/Legacy/Replay/Concurrent/ConcurrentCancelled/Tx2Disappeared/LateCancel） |
| order `-race` 同 | **PASS 13.370s** |
| fulfillment `TestCancelIfCreated` | **PASS 2.011s**（3 个用例） |
| 既有 `TestAdmitRefund*` 回归 | **PASS 21.005s** |
| `make proto-guard` | **[proto-guard] OK (1002 baseline entries)** |
| `migrations` diff | **空**（无迁移/回填） |

原始日志：`.tmp/evidence-ac50-t0/reviewer-t0.log`、`.tmp/evidence-ac50-t0/order-existing.log`、`.tmp/evidence-ac50-t0/proto-guard.log`（本次审查生成，未入库）。

---

## 2. T0（Fulfillment / proto）

| 检查 | 结论 | 证据 |
|---|---|---|
| 无行 → `NO_WAYBILL`，不建行/不 commit/不误取消 | **PASS** | `internal/services/fulfillment/cancel_if_created.go:19,64`；`TestCancelIfCreatedNoWaybill` |
| 真实 `CREATED→CANCELLED`/已 `CANCELLED` 保持 | **PASS** | `TestCancelIfCreatedOnlyCancelsCreated` |
| `SHIPPED/RECEIVED/EXCEPTION` 原值；`EXCEPTION` 无 `shipped_at` → `TRANSIENT_ERROR` | **PASS** | `TestCancelIfCreatedOnlyCancelsCreated`、`TestCancelIfCreatedRefusesExceptionAfterShipment` |
| `shipment_no` 收窄范围 + G1/G2 组隔离 | **PASS** | `TestCancelIfCreatedNoWaybill`（F1b/F6） |
| proto 注释 additive、无新字段/枚举/RPC；生成物仅注释 | **PASS** | `api/proto/mall/v1/mall.proto:1093-1106`；`api/gen/mall/v1/mall.pb.go:11200-11212`；proto-guard OK |
| wire token 与 ADR 占位记法 | **PASS（注记）** | ADR 0.6 §2.1 用 `<NO_WAYBILL>` 占位并允许实施 Brief 确认；代码/测试/proto 用 wire token **`NO_WAYBILL`**（无尖括号）。selfcheck §2 已说明；需 Lead/PO 在实施 Brief 记录该命名，见 §5 R3。 |

---

## 3. T0b（Order 映射 / 幂等 / 零元 / legacy）

| 检查 | 结论 | 证据 |
|---|---|---|
| `NO_WAYBILL` → Tx2 `result=REVIEW_REQUIRED,state=PENDING_REMOTE` | **PASS** | `refund_admission.go:678-682`；`TestAC50ReviewMapping`（DB v2、details result/state/reason/ticket/manual_queue/probe_status） |
| 不写 PRE/POST、无终态 outbox/release、无 `Payment.Refund` | **PASS** | `TestAC50ReviewMapping`（终态 outbox 0、release 0、`refundCalls==0`；Tx1 占位保留 1） |
| permit 保持 `CONSUMED`、commitment 保持 `RESERVED` | **PASS** | 同上 |
| 未知/未来 status → `Unavailable`（永不 PRE） | **PASS（代码）/ 缺测试** | `refund_admission.go:683-685` default branch；自检声明；**T0 测试未覆盖未知值**，见 §5 R1 |
| existing-first：Payment 读/远程探测之前读裁决；REVIEW 幂等、Payment 不可用也同错 | **PASS** | `refund_admission.go:219-237`；`TestAC50ReviewReplay`（getCalls/cancelCalls 不增、version 不变、O2c Payment down） |
| 零元 `NO_WAYBILL`：不写 admission/S19/`DONE`/REVIEW，不计 AC-72，不调 Payment | **PASS** | `refund_admission.go:472-484`；`TestAC50ReviewZeroNotPersisted`（admission/refund_applications/commitments/outbox 0、permit 不变、get/refund 0） |
| legacy 无 permit → PRE；CONSUMED+无行 → REVIEW；CONSUMED+已发 → POST | **PASS** | `TestAC50ReviewLegacy`（含 `cancelCalls==0` 的 NONE 分支） |
| typed details（禁止 message 嗅探） | **PASS** | `refund_admission.go:176-186`；`ac50RequireReviewConflict` 逐 key 断言 |

---

## 4. T0c（Tx2 双谓词 CAS）

| 检查 | 结论 | 证据 |
|---|---|---|
| CAS 精确为 `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'` | **PASS** | `refund_admission.go:702-705`（参数 `refundPending`/`refundReviewRequired`） |
| REVIEW 不被迟到 CANCELLED/并发 Tx2 洗成 PRE/POST | **PASS** | `TestAC50ReviewLateCancelDoesNotWash`、`TestAC50ReviewConcurrent`（O11/W5、O11d/W13） |
| 0 行回读：REVIEW→同错、PRE/POST→同结果、nil→`Conflict("admission disappeared")`、PENDING→retry；0 行无 outbox/release | **PASS** | `refund_admission.go:713-715,746-765`；`TestAC50ReviewTx2Disappeared` |
| 并发真实 CANCELLED：1 条终态 outbox、≤1 release、双方同 PRE | **PASS** | `TestAC50ReviewConcurrentCancelled`（O11b） |
| `-race` | **PASS** | 13.370s |

---

## 5. INV / AC / 残余与建议

- **INV-45**：T0 的 REVIEW 路径不改 permit（保持 `CONSUMED`）、不改 commitment（保持 `RESERVED`）、不 release；`TestAC50ReviewMapping` 断言。**PASS（T0 路径）**。
- **INV-51**：REVIEW 持久化、existing-first 幂等、不自动收敛；T0b/T0c 测试覆盖；TCC/AfterSales 镜像未落地（T1/T2）。**PASS（T0 路径）/ CONDITION（T1/T2）**。
- **INV-52**：partial 不产生取消副作用**不在 T0**；T0b 当前对 partial `CONSUMED+NO_WAYBILL` 仍会走探测并写 REVIEW，最终由 T3 的三分类替换。**T0 不验证 INV-52；不得据 T0 PASS 声称 AC-50/INV-52 全量通过**。

**后续建议（最多 3 条）**

1. **补未知/future probe status 测试**：Plan T0b expected 含“未知→`Unavailable`”，当前只有代码 default 分支、无 T0 用例；补 `TestAC50ReviewUnknownProbe`（不发 PRE/POST、不写 release/Refund），绑定 HEAD。
2. **T0 不单独发布**：T1（TCC `MANUAL_HOLD`）、T2（AfterSales canonical/8 次）、T3（partial 三分类/INV-52）未落地；若分批，必须 **Fulfillment 先、T0c 次、T0b 后**；新 Order + 旧 Fulfillment 会把无行当 `CANCELLED`→PRE，禁止。partial 在 T3 前不得上线。
3. **记录 wire token 命名**：ADR 0.6 §2.1 的 `<NO_WAYBILL>` 是占位；实现采用 `NO_WAYBILL`（proto/Order/Fulfillment/测试一致）。请在实施 Brief/Lead 记录该命名（无需改代码）；若要字面尖括号，需同步改三处常量/注释并重跑。

**不声称**：AC-50/AC-69/AC-70 的 AC-VERIFIED（仍 AC-REF/DEFERRED 归台账）、AC-72 后端段（T5）、Release PASS、merge；`PAY-SETTLE-01` 仍冻结。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-26
**报告**：`docs/collab/qc-reports/REV-PAY-AC50-REVIEW-01-T0.md`
