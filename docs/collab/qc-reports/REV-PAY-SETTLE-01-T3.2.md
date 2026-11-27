# REV-PAY-SETTLE-01-T3.2 — T3.2 人工释放（RESERVED→RELEASED）ops CLI 审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（在途；SPEC-0002 AC-62 / D-24 / INV-45） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | 新 `internal/services/order/refund_commitment_release.go`；新 `cmd/refund-commitment-release`；`refund_commitment_release_integration_test.go`；`t3.2-plan.md`；`settlement_integration_test.go` 的 drop 列表、`.gitignore`（测试卫生）。**不审** 168h hold、T3.4/T3.5 其余、T3.6、T5。 |
| base | **`4f7793c`**（`git rev-parse HEAD` = `4f7793c15e3958fd19ada012ec4b0152c9875159`） |
| head | **工作树 @ `4f7793c`（未提交）**：`refund_commitment_release.go`；`cmd/refund-commitment-release/main.go`；测试；plan；`.gitignore` 加 CLI 产物；drop 列表加 `refund_commitment_releases`。`migrations`/`api` 未改。 |
| spec_version | **SPEC-0002 v1.11**（approved 2026-11-26；AC-62/AC-60/AC-63） |
| adr_revision | **ADR-0007 Rev 2.3 / Amendment 1 §4.3/§6.4**（release 语义/表） |
| baseline_impact | **有**：新增唯一人工释放入口；只把 `RESERVED` 承诺额度移出，不改 permit/订单/组/Payment/退款事实；无 proto/迁移；ops CLI 形态，依赖部署控制。 |
| INV | **INV-45**（人工释放/禁止自动释放、只释放额度）、**AC-62**（人工授权/审计/幂等/CAS）、AC-60/63（仍不升级）。 |
| verification | **命令实测**：`go test ./internal/services/order -run 'TestReleaseCommitment' -v` → **7 PASS（6.122s）**；full `TestAdmitRefund\|TestTccAdmission\|TestLegacy\|TestReleaseCommitment` → **ok 63.232s**；`go vet ./internal/services/order ./cmd/refund-commitment-release` → 0；`go build ./cmd/refund-commitment-release` → OK；`git diff 4f7793c -- migrations api` = **0**；`refund_commitment_releases` 表在 `0018` 已存在。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **SCOPED-PASS（T3.2 ops CLI）**：只 `RESERVED→RELEASED`、D-24 字段/阈值/双人/过期/金额/幂等/并发/dry-run 通过；**“仅授权角色”依赖 ops 部署控制**（NB-1）；**AC-60/63 仍未验证**。0 阻断；NB-1…NB-5。 |

---

## 0. 结论

1. **只释放额度**：`ReleaseRefundCommitment` 只写 `refund_commitment_releases` + CAS `order_refund_commitments`；不写 permit/订单/组/Payment/refund_applications；无 worker/自动入口（生产代码只有 CLI 调用）。
2. **D-24**：release_id/authorization_id/operator/authorized_by/reviewed_by（≥100000 且 ≠ authorized_by）/reason_code/evidence_ref/authorized_at/expires_at 均校验；过期/缺字段/越权 → `REFUND_RELEASE_NOT_AUTHORIZED`；金额必须等于承诺头。
3. **“仅授权角色”**：ops CLI 无法做 IAM；`authorized_by/operator` 是审计身份，不是认证；依赖 ops 主机/DB 凭证/授权流程。**判定为可接受的部署控制（NB-1），不是阻断**（人类已批准 A=ops CLI）。
4. **Trigger**：封闭 4 枚举；`USER_WITHDRAW/BUSINESS_REJECT` 要求 `refund_applications` `REJECTED/CANCELLED`，无 application 行拒绝；`FAILED_PERMANENT/MANUAL_ABANDON` 允许。与 AC-62 两类触发一致；普通售后无 application 只能走后两者。
5. **幂等/并发**：同 release_id 同授权 → 首次结果；同 release_id 不同授权/同 refund 不同 release/同 authorization 不同 release → `REFUND_COMMITMENT_CONFLICT`；`FOR UPDATE` + `uk_release_refund/authorization` + CAS；并发两执行者只一个成功（测试）。
6. **DryRun/CLI**：dry-run 零写入（测试）；`-apply` 唯一写开关；配置错误 exit 2、业务错误 exit 1。
7. **边界**：`git diff 4f7793c -- migrations api` = 0；表在 0018；无新迁移；helper drop 列表/.gitignore 只是测试卫生。

---

## 1. 审查输入与方法

- 读：`refund_commitment_release.go`、CLI、7 个测试、`t3.2-plan.md`、`0018` 的 `refund_commitment_releases`、`ac50_review`/`refund_release` 既有 AC-62 证据。
- 跑：7 个 release 测试、full order 回归、vet、build、边界。
- 未改代码/测试/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ go test ./internal/services/order -run 'TestReleaseCommitment' -v
--- PASS: TestReleaseCommitmentSuccessAndIdempotent
--- PASS: TestReleaseCommitmentRejectsSettled
--- PASS: TestReleaseCommitmentSecondReview
--- PASS: TestReleaseCommitmentExpiredAndMismatch
--- PASS: TestReleaseCommitmentTrigger
--- PASS: TestReleaseCommitmentDryRunWritesNothing
--- PASS: TestReleaseCommitmentConcurrentOnlyOne
PASS  (6.122s)

$ go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy|TestReleaseCommitment' -count=1
ok   ...order  63.232s

$ go vet ./internal/services/order ./cmd/refund-commitment-release   # 0
$ go build ./cmd/refund-commitment-release                          # OK
```

---

## 3. AC-62/INV-45：只释放额度

- 生产代码 `ReleaseRefundCommitment` 的写操作只有：
  - `INSERT INTO refund_commitment_releases ...`；
  - `UPDATE order_refund_commitments SET state='RELEASED' ... WHERE state='RESERVED'`。
- 不写 `order_shipping_permits`/`orders`/`order_fulfillment_groups`/Payment/`refund_applications`；不写 `DONE`；不调用 Payment。
- 无 worker/自动入口：全仓 `ReleaseRefundCommitment` 只被 `cmd/refund-commitment-release` 调用（测试除外）；没有后台扫描/定时释放。
- **NB-4**：plan 的测试矩阵写“permit/订单/组不变”，但 `TestReleaseCommitmentSuccessAndIdempotent` 只断言 commitment/release 行，未显式断言 permit/订单/组不变；代码 inspection 是干净的，测试覆盖不足，非阻断。
- **NB-2（AC-62 对账窗口）**：AC-62 要求“Payment 已有退款成功事实但承诺仍 RESERVED → 先收敛 SETTLED，不得释放”。本工具只检查本地 commitment 状态，不查询 Payment；plan 的 stop_condition 也写明“若无法在不调用 Payment 的前提下排除…停止在该场景”。ops CLI 形态下需 **ops runbook 在 `-apply` 前核对 Payment 对账**（D-24 evidence_ref 记录）；工具本身不强制。判为部署控制 NB，不阻断 CLI 形态；若人类要求系统强制，则需另加只读 Payment 检查。

---

## 4. D-24 字段/阈值/双人/过期

`validateReleaseAuthorization` 校验：

- 必填：`release_id/refund_id/authorization_id/operator/authorized_by/reason_code/evidence_ref`；`amount_cent>0`；`authorized_at` 非空且不超过 now+1min；`expires_at` 非空且 > now。
- `reason_code` 封闭 4 枚举。
- `amount_cent >= 100000`：`reviewed_by` 非空、≠ `authorized_by`、`reviewed_at` 非空且在 [authorized_at, now+1min]；DB `chk_release_review` 兜底。
- 金额必须等于承诺头 `amount_cent`；order/group 若提供必须匹配。
- **NB-1（“仅授权角色”）**：CLI 不能做 IAM；`authorized_by/operator` 是审计字符串；角色由 ops 主机访问/DB 凭证/授权流程控制。**可接受的部署控制**（人类批准 A=ops CLI），不是代码阻断；生产需 ops 访问控制 + 双人授权 + 审计。
- **NB-3（reviewed_at 幂等缺口）**：`sameReleaseAuthorization` 比较 refund/amount/auth/operator/authorized_by/reviewed_by/reason/evidence/authorized_at/expires_at，但**不比较 `reviewed_at`**；`loadReleaseRow` 也未 SELECT `reviewed_at`。同 release_id 重试若只改 `reviewed_at`，会被当作幂等返回首次结果，release 行的 reviewed_at 保留首次值。D-24 审计字段，非阻断但应补。
- **NB-3b**：`ReviewedAt` 未参与幂等比较，同 refund 不同 release/authorization 的冲突检查正常（`uk_release_*` + 预查）。

---

## 5. Trigger（AC-62 两类）

- `USER_WITHDRAW/BUSINESS_REJECT`：查 `refund_applications`；无行 → `REFUND_RELEASE_NOT_AUTHORIZED`；state 非 `REJECTED/CANCELLED` → 拒绝；测试覆盖 BUSINESS_REJECT 无行/PENDING/REJECTED。
- `FAILED_PERMANENT/MANUAL_ABANDON`：只要求 D-24 字段，不查 application；普通售后无 application 只能用这两个。与 AC-62 ② “FAILED_PERMANENT 或人工放弃且取得人工授权”一致。
- **NB-5**：测试未覆盖 `USER_WITHDRAW`、`CANCELLED`、`FAILED_PERMANENT`、同 authorization 不同 release、group 不匹配、`reviewed_at` 幂等；代码路径存在，属测试缺口。

---

## 6. 幂等/并发

- 同 release_id 同授权 → `AlreadyReleased`，不重复写；同 release_id 不同授权 → 冲突；测试改 `ExpiresAt` 覆盖。
- 同 refund 不同 release：`loadRefundCommitmentReleaseByRefund` + `uk_release_refund`；并发测试两 release ID 只一个成功。
- 同 authorization 不同 release：`loadRefundCommitmentReleaseByAuthorization` + `uk_release_authorization`；测试未直接覆盖（NB-5）。
- `FOR UPDATE` 锁 commitment；CAS `WHERE state='RESERVED'`；`uk_release_refund/authorization`；并发两执行者只一个成功（测试）。
- **结论**：`FOR UPDATE` + 唯一键 + CAS 足够；无自动重试/worker。

---

## 7. DryRun / CLI

- `DryRun`：在校验授权/commitment/trigger 后、INSERT/UPDATE 前返回；测试断言 release 行 0、commitment 仍 RESERVED。
- CLI：`-apply` 默认 false，`DryRun: !*apply` 是唯一写开关；RFC3339 时间；配置/连接错误 `fatal` exit 2，业务错误 exit 1；`-amount-cent` 必填。
- 没有其他写 flag；dry-run 也会 `FOR UPDATE` 取锁但事务回滚，无净写入。

---

## 8. 边界与测试卫生

- `git diff 4f7793c -- migrations api` = **0**；`refund_commitment_releases` 表在 `0018_refund_commitment_composition.sql` 已存在；无新迁移。
- `settlement_integration_test.go` 的 drop 列表加 `refund_commitment_releases`：测试隔离所需，合理。
- `.gitignore` 加 `/refund-commitment-release`：CLI 构建产物，合理。
- `tasks.md` T3.2 仍 `[ ]`；T3.4/T3.5 仍 `[ ]`；T3.6 `[ ]`。T3.2 ops CLI 可在台账 PASS 后标 `[x]`（T3.2 范围），由作者更新。

---

## 9. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | “仅授权角色”无法由 CLI 认证；`authorized_by/operator` 是审计身份；依赖 ops 主机/DB 凭证/授权流程。**可接受部署控制**（人类批准 A=ops CLI），生产需 ops 访问控制+双人+审计 | `cmd/refund-commitment-release`；`validateReleaseAuthorization` |
| NB-2 | AC-62 对账窗口：工具只查本地 commitment 状态，不查 Payment；ops runbook 必须在 `-apply` 前核对 Payment，D-24 evidence 记录；plan stop_condition 已声明 | AC-62；`t3.2-plan.md` §5 |
| NB-3 | `sameReleaseAuthorization` 不比较 `reviewed_at`；`loadReleaseRow` 未 SELECT；同 release_id 只改 reviewed_at 会被当幂等 | `refund_commitment_release.go` |
| NB-4 | 测试未显式断言 permit/订单/组/Payment 不变（plan 矩阵有）；代码 inspection 干净 | `TestReleaseCommitmentSuccessAndIdempotent` |
| NB-5 | 测试缺 USER_WITHDRAW/CANCELLED/FAILED_PERMANENT、同 authorization 不同 release、group 不匹配、reviewed_at 幂等 | 7 个 release 测试 |

---

## 10. 裁决

**SCOPED-PASS（T3.2 ops CLI）。** 只 `RESERVED→RELEASED`、D-24 字段/阈值/双人/过期/金额/幂等/并发/dry-run 通过；无 permit/订单/组/Payment 写入；无 worker/自动入口；表已存在、无迁移。

**T3.2 ops CLI 可交人类合入；“仅授权角色”依赖 ops 部署控制；AC-60/63 仍未验证；NOT RELEASE。**

AC 位移：`docs/specs/0002` **AC-62 有引用 → 已验证**（T3.2 ops CLI；`TestReleaseCommitment*` 7 PASS；角色/对账窗口为部署控制 NB）；AC-60/AC-63 保持未验证。

---

## 11. 复现命令（只读）

```bash
git rev-parse HEAD   # 4f7793c...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestReleaseCommitment' -count=1 -v
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/order -run 'TestAdmitRefund|TestTccAdmission|TestLegacy|TestReleaseCommitment' -count=1
go vet ./internal/services/order ./cmd/refund-commitment-release
go build ./cmd/refund-commitment-release
git diff 4f7793c -- migrations api | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 SCOPED-PASS 裁决 + `docs/specs/0002` AC-62 验收状态。未改代码/测试/契约/迁移；未 commit、未 merge。
