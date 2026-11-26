# REV-PAY-AC50-REVIEW-01-PLAN — L2-P4 Plan 可验证性评审

| 项 | 值 |
|---|---|
| change_id | `PAY-AC50-REVIEW-01` |
| Tier | **L2（确认）**：跨 Fulfillment/Order、资金/履约准入、既有 RPC 响应语义、共享 metrics；不适用 L1 减免 |
| review_scope | **仅 L2-P4 Plan 可验证性**：Plan/Tasks 的 evidence path、AC 行、命令、HEAD 绑定、AC-69…72 台账/AC-72 三段、与 PAY-SETTLE-01 回归隔离。不审 SPEC/ADR 正文，不审实现 |
| 被审 Plan | P4：`plan.md` Base `d2149c0` / 文件提交 `752b3f5`；**P4b**：工作树重写版 Base `a1cc809`（见 §6） |
| 契约基线 | SPEC-0002 **v1.11 approved、ADR 0.5 ACCEPTED** 目前是**工作树未提交**（C-4 记录见 `docs/review-tracking.md:366-373`；提交 `70821c8`/`3c30b19` 仍是 in-review/DRAFT）；impact 0.3；QC R8 READY（`edebb85`） |
| Actor / Reviewed-by | `reviewer-r04` / `human (Plan P5 pending)` |
| release_status | **N/A（Plan 评审）**；PASS 只表示 Plan 可执行/可验证，不是实现/Release/AC-VERIFIED |
| overall | **P4 BLOCKED → P4b READY（conditional；见 §6）** |

## 0. 结论

**BLOCKED。** Plan 的任务描述尚不足以让 Reviewer **不读实现细节**就判定 PASS：L2-T1…T8 没有精确 evidence path、AC 行、测试文件/命令或 O/F/W 编号绑定；AC-72 的三段台账未在 Plan/Tasks 登记；Plan 仍把 ADR 0.5 写成“待接受”、C-4 未勾选，且 `allowed_paths` 与 ADR 0.5 不精确一致。另有一处与 SPEC AC-72 直接冲突的 BFF 文案任务。

## 1. `[BLOCKER]` 可验证性缺陷

1. **任务→证据映射缺失且违反“一任务一不变量”**：`plan.md:39-48` / `tasks.md:24-31` 的 L2-T1…T8 只写“TCC 集成/三分支集成/故障注入/真实 MySQL/-race”，没有测试文件、`go test` 命令、O/F/W 编号（ADR §10 已有 O1/O1b/O2/O3/O4b/O4c/O4d/O4e/O7/O19/O20/O20b/O21/O22/O23，`ADR:662-719`），也没有每任务绑定的 HEAD；且 T1/T2/T5/T6 各混合多个不变量，违反 `plan.md:37` 的“每任务独占一个不变量/分支”，Reviewer 无法按任务/AC 判 PASS。
2. **AC-69…72 台账/AC-72 三段未落 Plan**：`plan.md:47` / `tasks.md:30` 仅写“AC-69…72 + ac-coverage.tsv”；**未列** AC-72 后端段、`REVIEW-REQUIRED-ALERTS-01`、`REVIEW-REQUIRED-ENTRY-01` 三行（owner/expiry/status/evidence），而 SPEC AC-72 `:492` 与 QC R8-T2 `QC-PAY-SETTLE-01-AC50-NOWB.md:1329` 明确要求三段登记、AC-72 不得整体 VERIFIED。未登记 `O18`/`O21` 证据路径与 `mall_refund_review_backlog/oldest` 名称。
3. **BFF 文案与 SPEC 冲突**：`plan.md:45` 要求 L2 交付 BFF 文案“退款处理中（需人工核对）”；SPEC AC-72 `:492` 明确目标文案/人工入口归 `REVIEW-REQUIRED-ENTRY-01`（DEFERRED），“本 L2 不交付 BFF/UI 入口改造”，交付前沿用既有“退款处理中”。`tasks.md:28` 同。该项会把 deferred 范围错误纳入 L2，Reviewer 无法判定。
4. **C-4/ADR/HEAD 绑定过期**：`plan.md:5` 仍写 DRAFT、`:12` 写 ADR 0.5“待接受”、`:33` C-4 未勾选，`tasks.md:9-12` C-1…C-4 全未勾选；实际 C-4/SPEC/ADR 的批准状态在**工作树未提交**（`review-tracking:366-373`、SPEC/ADR worktree diff），提交 `70821c8`/`3c30b19` 仍是 in-review/DRAFT。Plan Base `d2149c0`（`:10`）早于 C-4，且 `:11` Spec-Ref 只列 D-30/31，漏 **D-32（审计）/D-33（AC-72 分段）/D-35（canonical 截断）**；Plan 未绑定 SPEC/ADR/impact 的 worktree blob 或提交。
5. **`allowed_paths` 与 ADR 0.5 不精确一致**：`plan.md:14` 把 `internal/services/order/` 写成宽表“必要的 `refund_batch*`/consumer/metrics”，**漏 `internal/services/order/worker.go`**（ADR `:24` 明确 AC-72 gauge 刷新需要），且“metrics”未写清是 `internal/platform/metrics`；`:15` forbidden 漏 `docs/implementation/**`、`docs/ops.md`、`scripts/**`、`Makefile`、accepted ADR 正文等 ADR `:25` 所列项。实施 Brief 无法据此判越界。

## 2. `[MAJOR]` 语义断言不可核

- **canonical 格式**：`plan.md:42` / `tasks.md:25` 写 `REVIEW_REQUIRED reason=<code> ticket=<id>`；SPEC AC-72 `:496-500` 要求 `ticket=RVW:<refund_id>`、无 `arm`、确定性截断 ` truncated=1`。Plan 的泛化格式无法断言 O19/O20/O23。
- **部分金额**：`plan.md:43` 的“未离仓→PARTIAL **零写入**”漏 preseed 分支（SPEC AC-70 `:474` / ADR O4e `:689`：已有 `PENDING_REMOTE` 保留既有 admission/commitment、不新增写入）；fresh 与 preseed 必须分开。
- **“0 Payment”**：`plan.md:41` / `tasks.md:24` 的“0 Payment”过宽；ADR O1b `:679` 要求“0 打款”，O1 只断言 `refundCalls==0`，`GetIntent` 可能已发生。应按“0 `Payment.Refund/RefundBatch` / 0 打款”断言。

## 3. 与 PAY-SETTLE-01 的回归隔离

- `plan.md:48` 写“回归 AC-14/21/25/27/31/32、INV-44…50”，同句又写“**不回归 PAY-SETTLE-01 基线**”，与 `plan.md:18`（回归基线）和 `:68`（AC-14 等不得回退；AC-27 仍 AC-REF）冲突。应改写为“**必须跑**这些基线回归；**不得**关闭/推进 PAY-SETTLE-01 的 AC-27/Phase 0-R 状态”。否则 Reviewer 无法判定“回归”到底是运行还是禁止。

## 4. READY 条件（P5 前必须补齐）

1. 在 Plan/Tasks 为 L2-T1…T8 逐项写明：**测试文件/命令**、**O/F/W 编号**、**AC 行号**、**精确 HEAD/blob**；至少映射 O1/O1b/O2/O3/O4b/O4c/O4d/O4e/O7/O18/O19/O20/O20b/O21/O22/O23。
2. 落 **AC-72 三段台账**（后端段 / `REVIEW-REQUIRED-ALERTS-01` / `REVIEW-REQUIRED-ENTRY-01`）的 owner/expiry/status/evidence；AC-69…72 各自行与 AC-50/51/61/62/65/67 修订行。
3. 删除 L2 的 BFF 目标文案；只保留 `gateway` 既有显示映射（或明确归 ENTRY-01），并按 SPEC AC-72 沿用“退款处理中”。
4. 更新 Plan 元数据：C-1…C-4 已闭合、ADR 0.5 ACCEPTED、SPEC v1.11 approved、Plan/SPEC/ADR/QC 的精确 blob/HEAD；补 D-32/D-33/D-35；`allowed_paths/forbidden_paths` 与 ADR 0.5 `:24-25` 逐字对齐（含 `order/worker.go`、`internal/platform/metrics/**`）。
5. 修正 canonical、preseed partial、0 Payment 断言与回归隔离措辞。

**Tier L2 确认；当前 Plan 不可交 P5。**

---

## 6. P4b focused re-review（2026-11-26，工作树 Plan/Tasks）

**范围**：只复核上一轮 P4 的 blocker/MAJOR；Plan/Tasks 工作树，Base `a1cc809`，C-5 ADR 0.6 errata 待人类批准。

| 上轮项 | P4b 判定 | 依据 |
|---|---|---|
| B1 任务→证据/命令/O 码/HEAD、一任务一不变量 | **CLOSED（残余 MINOR）** | `plan.md:39-62` 每任务已有 ADR §10 码、测试文件、命令、expected；T1/T2/T5/T6 已拆原子。T1a/T1b/T2a/T3/T4a/T5a/T6a/T6d 等命令仍用 `...` 省略 env 前缀，见残余 R1。 |
| B2 AC-72 三段/AC-69…72 台账 | **CLOSED（残余 MINOR）** | `plan.md:71-75` 有 AC-72 后端/`REVIEW-REQUIRED-ALERTS-01`/`REVIEW-REQUIRED-ENTRY-01` 三段，`plan.md:61` T7 与 `tasks.md:72` 要求 3 段 tsv、不得整体 VERIFIED；exact owner/expiry/status 由 T7 填，见 R2。 |
| B3 BFF 文案冲突 | **CLOSED** | `plan.md:14-15` 已移除 gateway；`plan.md:73-75` 保留 ENTRY-01 deferred；ADR 0.6 `:24-25` 删除 gateway 并允许 `docs/ops.md` runbook。 |
| B4 C-4/ADR/HEAD 过期 | **CLOSED（残余 CONDITION）** | `plan.md:5,10-12,26-33`、`tasks.md:9-13` 已勾 C-1..C-4，Base `a1cc809`，Spec v1.11 approved/ADR 0.5 ACCEPTED；C-5 ADR 0.6 errata pending human，见 R4。 |
| B5 allowed_paths 与 ADR 不一致 | **CLOSED（残余 MINOR）** | `plan.md:14` 已含 `order/worker.go`、`internal/platform/metrics/metrics.go`、`docs/ops.md`；与 ADR 0.6 `:24-25` 基本对齐。仍有两处待对齐，见 R3。 |
| M1 canonical/partial/0 Payment | **CLOSED** | `plan.md:50` T2b canonical `ticket=RVW:<refund_id>`；`:52` T3 fresh/preseed；`:46,49,53` `refundCalls==0`/`getCalls` 区分；`tasks.md:46-57` 同。 |
| M2 回归隔离 | **CLOSED** | `plan.md:18` “必须跑；不得改 PAY-SETTLE-01 的 AC-27/Phase 0-R 状态”；`tasks.md:73` 同。 |

### 残余（不构成 P4 BLOCKER，P5 前处理）

- **R1 [MINOR] 命令仍有 `...`**：`plan.md:44-60`、`tasks.md:34-71` 多条 `... -run`，不是可直接复制的命令；补 `TEST_MYSQL_DSN=$ORDER_DSN/...` 前缀与包路径。测试文件/O 码/expected 已齐。
- **R2 [MINOR] AC-72 三段 tsv 行**：`plan.md:74` 只列段名与 DEFERRED；T7 需落三行 `ac-coverage.tsv` 的 owner/expiry/status/evidence_path，AC-72 不得整体 VERIFIED。
- **R3 [MINOR] 两处 path 对齐**：`plan.md:14` 允许 `docs/implementation/PAY-AC50-REVIEW-01-selfcheck.md`，但 ADR 0.6 `:25` forbidden 含 `docs/implementation/**`；`plan.md:14` 未列 ADR 0.6 `:24` 允许的 `api/gen/mall/v1/{mall.pb.go,mall_grpc.pb.go}` 生成物。P5 前在 Plan/ADR 二选一补齐。
- **R4 [CONDITION] C-5**：ADR 0.6 errata 仍待人类 `ADR_APPROVED`（`plan.md:5,33`；`tasks.md:13`）。C-5 未关不写码；P5 前确认。

**P4b verdict：READY（conditional）**——上一轮 P4 blocker 均已关闭；无新的 P4 blocker。P5 前完成 R1–R4（文本/台账/path/C-5），并保留 `NOT RELEASE`、AC-27 AC-REF、不关父状态。
