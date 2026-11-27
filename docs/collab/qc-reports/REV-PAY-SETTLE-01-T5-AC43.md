# REV-PAY-SETTLE-01-T5-AC43 — T5.3 AC-43 Payment settlement crash/resume 审查

| 项 | 值 |
|---|---|
| change_id | **`PAY-SETTLE-01`**（T5） |
| 级别 | **实现变更** |
| 阶段 | **代码** |
| review_scope | `internal/services/payment/settlement.go` 的 `claimSettlementTask`（PENDING/PROCESSING + lease CAS）；`settlement_task_integration_test.go` 两个新测试。**不审** AC-43 的 CANCELLED 全额退款/HELD-CONFIRMED 收敛、AC-59 第 4 条。 |
| base | **`aad98a1`** |
| head | **工作树 @ `aad98a1`（未提交）**：settlement.go +9/-4；test +85；无 proto/migration。工作树另有未跟踪 `docs/archive/2026-11-27-governance/{AGENTS_archive,constitution_archive}.md`，**非本次变更**。 |
| spec_version | **SPEC-0002 v1.11**（AC-43；AC-59/AC-60/63 不变） |
| adr_revision | **ADR-0007 Rev 2.3 / Amd1** |
| baseline_impact | **有**：崩溃后 PROCESSING 任务 lease 过期可恢复；max attempts 到 FAILED_PERMANENT；只 payment；无 proto/migration。 |
| INV | **AC-43**（settlement crash/resume）、AC-59/AC-60/63（不变）。 |
| verification | **命令实测**：`go test ./internal/services/payment -run 'TestSettlementTask' -v` → **3 PASS（2.155s）**；`go test ./internal/services/payment -run 'TestSettlement'` → **PASS（2.149s）**；`go vet ./internal/services/payment` → 0；`git diff aad98a1 -- api migrations` = **0**。 |
| Actor / Reviewed-by | **`reviewer-syntax`**（独立 subagent，与作者 `lead` 不同 actor） / **`human`**（待批准；作者 `lead` 不作 Reviewed-by） |
| release_status | **NOT RELEASE**；不批准 merge/Release |
| overall verdict | **PASS（AC-43 crash/resume + max attempts）**：PROCESSING lease 过期可恢复；活跃 lease 不被抢（CAS 条件）；max attempts → FAILED_PERMANENT；正常 PENDING claim/finish/release 未受影响。**AC-43 部分；T5.3 仍部分；AC-60/63 不变**。0 阻断；NB-1…NB-5。 |

---

## 0. 结论

1. **过期 PROCESSING 可恢复**：`claimSettlementTask` SELECT 改 `state IN (PENDING,PROCESSING)` 且 `lease_until IS NULL OR lease_until < NOW(3)`；UPDATE 的 CAS 条件与 SELECT 同构，lease 过期后可重新 claim。
2. **活跃 lease 不被抢**：`lease_until` 未过期的行不满足 SELECT/UPDATE 条件；`FOR UPDATE SKIP LOCKED` + CAS 保证一个 owner。
3. **max attempts**：10 次 claim/release 后 `FAILED_PERMANENT`，attempts=10；测试 PASS；无重复 settlement（任务从未 finish/Payment）。
4. **正常 PENDING**：`TestSettlementTaskClaimFinish` 与 `TestSettlement` 全 PASS；PENDING claim/finish/release 未变。
5. **AC-43 仍部分**：只做 crash/resume + max attempt；CANCELLED 全额退款/HELD-CONFIRMED 收敛未做；AC-43 不能标已验证。
6. **AC-59 第 4 条未做**：库存失败先于支付成功仍缺 Inventory 失败事件契约；AC-59 3/4；T5.3 仍部分；AC-60/63 不变。

---

## 1. 审查输入与方法

- 读：settlement.go diff、新测试、`releaseSettlementTask`/`finishSettlementTask`/`settlementMaxAttempt`、`TestSettlementTaskClaimFinish`。
- 跑：payment 测试、settlement 回归、vet、边界。
- 未改代码/契约/迁移；未 commit/merge。

---

## 2. 测试

```text
$ TEST_MYSQL_DSN=... go test ./internal/services/payment -run 'TestSettlementTask' -count=1 -v
--- PASS: TestSettlementTaskClaimFinish (0.18s)
--- PASS: TestSettlementTaskCrashResume (1.46s)
--- PASS: TestSettlementTaskMaxAttemptsFailsPermanent (0.40s)
PASS  (2.155s)

$ TEST_MYSQL_DSN=... go test ./internal/services/payment -run 'TestSettlement' -count=1
ok   ...payment  2.149s

$ go vet ./internal/services/payment   # 0
$ git diff aad98a1 -- api migrations | wc -l   # 0
```

---

## 3. 过期 PROCESSING / 活跃 lease / CAS

- SELECT：`state IN (PENDING,PROCESSING) AND (lease_until IS NULL OR lease_until < NOW(3))`；`FOR UPDATE SKIP LOCKED`。
- UPDATE：`WHERE payment_no=? AND state IN (PENDING,PROCESSING) AND (lease_until IS NULL OR lease_until < NOW(3))`，SET `PROCESSING`/owner/lease/version+1；与 SELECT 同构。
- 活跃 lease：`lease_until >= NOW` 不满足条件，不会被抢。
- **NB-1**：新测试只覆盖“过期后恢复”，没有“活跃 lease 时第二个 claim 返回 nil”的断言；代码路径正确但未实测。
- **NB-2**：UPDATE 后未检查 `RowsAffected`；`FOR UPDATE` 已锁行，CAS 条件同构，实际为 1；建议加防御性检查。

---

## 4. max attempts / 无重复 settlement

- `releaseSettlementTask` 每次 attempts+1；到 `settlementMaxAttempt`（10）→ `FAILED_PERMANENT`。
- 测试：10 次 claim/release，state=FAILED_PERMANENT、attempts=10。
- 任务从未 finish/Payment，故无重复 settlement；`TestSettlement` 正常路径 PASS。
- **NB-3**：max attempts 测试未断言“无 settlement/Payment 调用”；当前靠任务未 finish；建议补。

---

## 5. 正常 PENDING / AC-43 / AC-59

- `TestSettlementTaskClaimFinish`：PENDING claim→finish DONE；未受 PROCESSING 分支影响。
- `TestSettlement` 回归 PASS；正常 PENDING claim/finish/release 未变。
- **AC-43 部分**：crash/resume + max attempt；CANCELLED 全额退款/HELD-CONFIRMED 收敛未做；不能标已验证。
- **AC-59 第 4 条未做**：Inventory 失败先于支付成功仍缺契约；AC-59 3/4；T5.3 仍部分。
- **AC-60/63 不变**：AC-60/63 保持已验证。

---

## 6. 边界

- `git diff aad98a1 -- api migrations` = **0**；无 proto/migration。
- 只改 `internal/services/payment/settlement.go` + test；无其他契约越界。
- 工作树未跟踪 `docs/archive/2026-11-27-governance/{AGENTS_archive,constitution_archive}.md` 不属本次变更，未触碰。

---

## 7. NB（非阻断）

| # | 问题 | 依据 |
|---|---|---|
| NB-1 | 无“活跃 lease 第二个 claim 返回 nil”测试 | `TestSettlementTaskCrashResume` |
| NB-2 | UPDATE 后未检查 `RowsAffected` | `claimSettlementTask` |
| NB-3 | max attempts 测试未断言无 settlement/Payment | `TestSettlementTaskMaxAttempts...` |
| NB-4 | 测试 sleep 1.2s vs lease 1s，存在时序余量风险 | `TestSettlementTaskCrashResume` |
| NB-5 | AC-43 的 CANCELLED/HELD-CONFIRMED、AC-59 第 4 条未做 | AC-43/AC-59 |

---

## 8. 裁决

**PASS（AC-43 crash/resume + max attempts）。** PROCESSING lease 过期可恢复；活跃 lease 不被抢（CAS 条件）；max attempts 到 FAILED_PERMANENT；正常 PENDING claim/finish/release 未变；无 proto/migration。

**AC-43 部分；T5.3 仍部分；AC-60/63 不变；NOT RELEASE。**

---

## 9. 复现命令（只读）

```bash
git rev-parse HEAD   # aad98a1...
git status --porcelain
export PATH="$PWD/.tmp/bin:$PATH" GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp \
  GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/payment -run 'TestSettlementTask' -count=1 -v
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true' \
  go test ./internal/services/payment -run 'TestSettlement' -count=1
go vet ./internal/services/payment
git diff aad98a1 -- api migrations | wc -l   # 0
```

**本次落盘**：本报告 + `docs/review-tracking.md` 最新块顶部一条 PASS 裁决。未改代码/契约/迁移；未 commit、未 merge。
