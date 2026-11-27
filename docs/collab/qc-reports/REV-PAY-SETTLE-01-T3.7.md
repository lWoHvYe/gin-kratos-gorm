# REV-PAY-SETTLE-01-T3.7 — T3.4/T3.5/T3.7 收口（仅测试）审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | 新增 `internal/services/order/t3_7_refund_admission_integration_test.go`（3 个测试）；T3.4/T3.5/T3.7 的收口证据。**仅测试新增**，无生产代码/迁移/契约。 |
| base | **`4a743ee`**（`git rev-parse HEAD` = `4a743ee56503768948de90783f478f32a942a99c`） |
| head | **工作树 @ `4a743ee`（未提交）**：仅新增 `t3_7_refund_admission_integration_test.go`；`git diff 4a743ee -- 'internal/services/order/*.go'` = 0；无生产代码/migration/api/specs 改动。 |
| spec_version | **SPEC-0002 v1.11**（AC-60/AC-63 仍不升级；T3.4/T3.5/T3.7 无独立 AC） |
| adr_revision | **ADR-0007 Rev 2.3 / Amendment 1**（T3 实现语义） |
| baseline_impact | **不涉及**：仅新增测试；无生产行为、迁移、proto、契约改动。 |
| INV | **INV-24**（行级/组级/captured）、**INV-47**（组成/责任/运费）、AC-60/63（仍不升级）。 |
| verification | **命令实测**：3 个新测试 `-v` → **PASS（1.996s）**；full `TestAdmitRefund\|TestTccAdmission\|TestLegacy\|TestReleaseCommitment` → **ok 69.961s**；AfterSales `TestAC14\|TestAC31\|TestAC32\|TestAC33\|TestFreightResponsibilityDisabled` → **ok 3.623s**（含 168h hold timeout）；`go vet ./internal/services/order` → 0；`git diff 4a743ee -- 'internal/services/order/*.go'` = 0。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（收口）**：3 个新测试覆盖并发同一行/known=0 amount-only/mixed 缺口；168h 由既有 `TestAC31/32` 覆盖；T3.6 映射已接生产路径；**T3.4/T3.5/T3.7 可标 `[x]`**；**AC-60/63 仍未验证**。0 阻断；NB-1/NB-2。 |

---

## 0. 结论

1. **三缺口已覆盖**：
   - `TestAdmitRefundConcurrentSameLine`：两个不同 refund_id 同一行各 6000，只 1 个成功、1 admission/1 commitment/1 line；
   - `TestAdmitRefundLegacyCompositionAmountOnlyAllowed`：prior known=0 组仍允许整组 amount-only 4000，落 amount-only 承诺；
   - `TestAdmitRefundMixedLegacyCompositionRejectsLines`：known=0+known=1 混合组带 lines fail-closed，0 admission。
2. **并发测试不是假阳性**：`safeLegacyPayment.GetIntent` 固定 CapturedCent=10000（有 mutex）；`AdmitRefund` 的 `findOrderForUpdate` 锁 orders 行；先成功者写 6000，后到者行/组剩余 4000，无法再写 6000；断言 1 success + 1/1/1 行，不是只断言“有错误”。
3. **168h/T3.6**：`TestAC31/32` 覆盖 168h hold timeout/default merchant；T3.6 `processRefundTask` 已把 responsibility/freight 接到 Order（`TestBuildRefundAdmissionRequest*` + AfterSales 定向测试）。T3.4 的 168h 证据充分；T3.6 映射覆盖生产路径。
4. **T3.4/T3.5/T3.7 可标 `[x]`**：行组成/commitment lines（Phase 1）、responsibility/reason/quantity=0/UNDETERMINED（Phase 2）、AfterSales 映射（T3.6）、release（T3.2）、并发/known=0/mixed（T3.7）均有测试；T3.4/T3.5 的其余子项由既有代码+测试覆盖。
5. **AC-60/63 仍未验证**：T3.4/T3.5/T3.7 的收口不等于 AC-60/63 全条；仍等 `TestAC60/TestAC63`/T5 证据；不升级。
6. **边界**：只有新增 `_test.go`；生产代码/迁移/契约零改动。

---

## 1. 审查输入与方法

- 读：`t3_7_refund_admission_integration_test.go`、既有 `TestAdmitRefund`/`TestTccAdmission`/`TestLegacy`/`TestReleaseCommitment`、AfterSales `TestAC31/32`、T3.6 映射报告。
- 跑：3 个新测试、full order 回归、AfterSales 定向（168h）、vet、边界。
- 未改代码/测试/契约/迁移（本报告只更新 tasks 的 T3.4/T3.5/T3.7 进度）；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/services/order -run 'TestAdmitRefundConcurrentSameLine|TestAdmitRefundLegacyCompositionAmountOnlyAllowed|TestAdmitRefundMixedLegacyCompositionRejectsLines' -v
--- PASS: TestAdmitRefundConcurrentSameLine (0.60s)
--- PASS: TestAdmitRefundLegacyCompositionAmountOnlyAllowed (0.53s)
--- PASS: TestAdmitRefundMixedLegacyCompositionRejectsLines (0.76s)
PASS  (1.996s)

$ go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy|TestReleaseCommitment' -count=1
ok   ...order  69.961s

$ go test ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestAC33|TestFreightResponsibilityDisabled' -count=1
ok   ...aftersales  3.623s
```

---

## 3. 三个测试是否覆盖缺口

- **并发同一行**：真实 DB；两个不同 refund_id；同一 line 1（paid 10000）；各 6000；`AdmitRefund` 先锁 orders 行；一个成功写 6000，另一个看行/组剩余 4000 失败；断言 1 success、1 admission、1 commitment、1 line。覆盖“两个不同 refund 抢同一行”，不是 happy path。
- **known=0 amount-only**：prior known=0 SETTLED 1000；新整组 amount-only 4000；`validateRefundComposition` 无 lines → 不触发 `UnknownComposition`；承诺落 `amount=4000, composition_known=0`。覆盖 A-03 的“整组可、部分行级不可”。
- **mixed known=0+1**：known=0 + known=1（带 line 1）；新请求带 lines；`loadTccGroupComposition` 置 `UnknownComposition=true`，fail-closed；0 admission。覆盖混合组规则。
- 三个测试均断言 DB 状态/行数，不是只断言 error。

---

## 4. 并发测试假阳性检查

- `safeLegacyPayment` 的 `GetIntent` 固定返回 `CapturedCent=10000`；`ListOrderIntents`/`Refund` 有 mutex；无共享切片竞态。
- `AdmitRefund` 在 Tx 内 `findOrderForUpdate` 锁 orders 行；后到者必须等锁；锁后 `loadTccGroupComposition` 读到先成功者的 RESERVED line 6000，行剩余 4000 / 组剩余 4000，6000 被拒。
- 断言 `successes == 1` + `admissions/commitments/lines == 1`；不是“只要有一个 err”就通过，也不是 fake 返回错误导致的假阳性。
- **NB-1**：测试未断言失败者的错误文本（是行剩余/组剩余/冲突）；成功数+行数已证明 invariant，错误文本属诊断，不阻断。

---

## 5. 168h / T3.6 证据

- `TestAC31FreightHoldTimeout` / `TestAC32FreightHoldTimeoutDefaultsToMerchant`：覆盖 168h hold 到期/默认商家，既有的 AfterSales worker；本次定向 PASS（3.623s）。
- T3.6 `processRefundTask` 已通过 `refundAdmissionRequest` 传 lines/responsibility/freight；`TestBuildRefundAdmissionRequest*` 4 个纯测试 + `TestAC14/30/33` 定向 PASS（见 `REV-PAY-SETTLE-01-T3.6.md`）。
- T3.4 的 168h 证据由既有 `TestAC31/32` 承担；T3.6 覆盖 responsibility/freight 生产路径；T3.4/T3.5/T3.7 收口成立。

---

## 6. 任务推进

- `tasks.md`：**T3.4/T3.5/T3.7 可从 `[ ]` 推进为 `[x]`**（本报告按用户授权更新进度；不改实现）。
- 已 `[x]`：T3.2、T3.3、T3.6。
- **AC-60/63 仍未验证**：T3 收口不等于 AC-60/63 全条；仍等 `TestAC60/TestAC63`/T5。
- **NB-2**：T3.2 任务行写“9 tests”，T3.2 实际 release 测试 7 个（`TestReleaseCommitment*`）；文档计数需作者校正，不影响 T3.7。

---

## 7. 边界

- `git diff 4a743ee -- 'internal/services/order/*.go'` = **0**；`git status` 仅新增 `t3_7_refund_admission_integration_test.go`；无生产代码/迁移/api/specs 改动。
- 无 `.dsh/.specify` 变更纳入本次范围（base 之后无相关改动）。
- 未改代码/契约/迁移；未 commit/merge。

---

## 8. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 并发测试未断言失败者错误文本（行/组剩余/冲突）；成功数+行数已证明 invariant | `TestAdmitRefundConcurrentSameLine` |
| NB-2 | `tasks.md` T3.2 写“9 tests”，实际 T3.2 release 测试 7 个；文档计数需校正 | `tasks.md:96`；`TestReleaseCommitment*` |

---

## 9. 裁决

**PASS（T3.4/T3.5/T3.7 收口，仅测试）。** 三个新测试覆盖并发同一行、known=0 amount-only、mixed 缺口；并发测试的 fake/行锁/计数可靠；168h 由既有 `TestAC31/32` 覆盖；T3.6 映射已接生产路径；边界仅新增 `_test.go`。

**T3.4/T3.5/T3.7 可标 `[x]`；AC-60/63 仍未验证；NOT RELEASE。**

---

## 10. 复现命令（只读）

```bash
git rev-parse HEAD   # 4a743ee...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAdmitRefundConcurrentSameLine|TestAdmitRefundLegacyCompositionAmountOnlyAllowed|TestAdmitRefundMixedLegacyCompositionRejectsLines' -count=1 -v
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy|TestReleaseCommitment' -count=1
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/aftersales -run 'TestAC14|TestAC31|TestAC32|TestAC33|TestFreightResponsibilityDisabled' -count=1
go vet ./internal/services/order
git diff 4a743ee -- 'internal/services/order/*.go' | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决 + `tasks.md` 的 T3.4/T3.5/T3.7 进度。未改代码/测试/契约/迁移；未 commit、未 merge。
