# QC-PAY-SETTLE-01-SPEC-0001-V15-R2 — SPEC-0001 v1.5 整改 delta 复核（R1 BLOCKED 关闭情况）

| 项 | 值 |
|---|---|
| change_id | `PAY-SETTLE-01` |
| 任务 | QC round 2：只审 SPEC-0001 v1.5 **相对 R1 被审版本的整改 delta**，逐条核对 R1 的 B-1/B-2/B-3、M-1…M-9、T-1…T-5 是否关闭 |
| 被审对象 | `docs/specs/0001-fulfillment-quantity-split.md` v1.5 整改稿（工作树 blob `73271a64a8677d175e53a08be72fe597e04ae738`；**未 commit**） |
| 结论 | **READY**（0 `[BLOCKER]`；R1 的 17 项全部关闭或按 §16.1 登记为有 owner/expiry 的延期；另附 3 个实现前条件 N-1…N-3 + 1 个 `[TODO]` N-4） |
| 日期 | **2026-11-26** |
| Actor | `qc-po-v15-r2`（Quality Checker；与 PO、Architect、Developer、Reviewer 不同 actor） |
| base | `110a0ff1909984835a35f2a5225a2feb82434605`（`docs(qc): SPEC-0001 v1.5 BLOCKED …`，即 R1 报告落盘） |
| head | `c432ed613b24461756fdfc75ac6da461684e4afd`（`git rev-parse HEAD`；SPEC v1.5 仍为工作树改动，见 §6） |
| Tier | **L2**（Spec 修订：状态机 / 退款准入 / 许可 / 读模型） |
| review_scope | **只审整改 delta**：AC-27/AC-27A、§4.1/§4.2、§6.6.1/§6.7、§7.1/§7.2/§7.5、§8/§9、§13/§14、§16（含 16.1 整改矩阵 / 16.2 前置缺陷 / 16.3 交付绑定）；以 R1 报告（`QC-PAY-SETTLE-01-SPEC-0001-V15.md`）的 17 项为清单逐条核对。只读交叉核对 SPEC-0002 v1.10 AC-48/AC-50/AC-62、INV-45，ADR-0007 Rev 2.3 §5.2/§5.3/§5.7。**不重审全文**，不审 R0.4 实现（当前工作树另含 `cmd/fulfillment/**`、`internal/platform/config/**`、`internal/services/fulfillment/**`、`migrations/order/0022_r04_checks.sql`、`deploy/**`、`scripts/**`、`docs/ops.md` 等 R0.4 改动，属其他 change 范围）。 |
| release_status | **NOT RELEASE / NOT RELEASABLE**；不批准 merge；不声称测试已执行；不声称 AC-VERIFIED；本报告不代替人类批准。 |

## 0. 结论

**READY。** R1 的 3 个 `[BLOCKER]` 均已在 v1.5 整改稿中关闭：

1. **B-1 CLOSED**：AC-27A 已改为只使用 Order 自有持久事实（`0001:616-625`），删除读模型中的 `CONSUMED+无运单 → REFUND_ADMISSION_CONFLICT`/告警字段；`CONSUMED+无运单` 归 AC-50 写路径（`0001:294`、§16 V15-3、`AC-50-FIX-01`），并明确“部分金额先返回 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”。AC-27A 不再要求 Order 观测运单。
2. **B-2 CLOSED**：AC-27 EARS-2 的合法生命周期枚举已补 `CANCELLED`（`0001:599-603`），`PENDING_PAYMENT` 明确不适用，并新增“未支付取消/TCC 已捕获 → `CANCELLED` + 组关闭共存”EARS（`0001:604-606`），与 AC-25/§7.1/AC-51/AC-65 一致。
3. **B-3 CLOSED（契约）**：`order_refund_admissions.result` 被明确为唯一持久载体（`0001:622`、AC-27A 注 `0001:643`）；登记 `AC-50-FIX-01`（写路径/零元路径/ADR §5.3 amendment）、`AC-27-NOWB-01`（人工终态/owner/expiry）、`AC-27-LIVE-01`（DLQ/重放），并规定未关闭前不得 AC-VERIFIED（§16.2、`0001:866`）。执行细节见 N-3。

R1 的 M-1…M-8、T-1/T-2/T-4/T-5 已关闭；M-9（commit/blob 绑定）与 T-3（AC-25/26 EARS 迁移债）按 §16.1 登记为 `[DEFERRED]`，owner/expiry 明确。**本轮没有新的 BLOCKER**；但整改引入了 3 个实现前条件（N-1…N-3）与 1 个 `[TODO]`（N-4），必须在 v1.5 commit/实现前由 PO（必要时 Architect/ops）闭合。

**证据限制（重要）**：上一被审 blob `efad166…` 已不在 git object DB（`git cat-file -t` 失败），无法做字节级 diff。本 R2 的 delta 判定依据 = R1 报告的逐条引文 + §16.1 整改矩阵 + 当前文本核读；不构成对未改动全文的重新验收。被审 v1.5 仍为工作树（未 commit），本报告绑定 blob `73271a6…`。

**只读声明**：未执行 `go test`/`-race`/MySQL/迁移；未修改 SPEC/ADR/代码/迁移/台账；只新建本报告；未 commit。数据/实现行为判断标 `[不确定]`。

---

## 1. R1 问题关闭表

| R1 ID | R2 状态 | 证据（当前 SPEC / 交叉核对） | 残余 |
|---|---|---|---|
| **B-1** AC-27A 读模型越权/错误码越界 | **CLOSED** | `0001:616-625`（只 Order 自有持久事实；删除读模型 CONFLICT/告警；“不得在 AC-27A 读模型路径输出 AC-50 写路径错误码”）；`0001:294`（无运单归 AC-50 写路径）；§16 V15-3（部分金额先 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`） | 无（新分支见 N-1） |
| **B-2** EARS-2 漏 `CANCELLED` | **CLOSED** | `0001:599-606`（枚举含 `CANCELLED`；`PENDING_PAYMENT` 不适用；新增取消共存 EARS）；`0001:112`（S1 6 值）、`0001:129`（AC-25）、`0002:236`/`0002:377`（AC-51/65） | 无（3 组措辞见 N-2） |
| **B-3** PRE 证据载体 / `CONSUMED+无运单` 人工终态 | **CLOSED（契约）** | `0001:622`/`0001:643`（唯一载体 `order_refund_admissions.result`；`state=PENDING_REMOTE` 是中间态）；§16.2 `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01`（owner/expiry；“未修前不得 AC-VERIFIED”`0001:866`） | 执行细节见 **N-3** |
| **M-1** S4/S9 编号 | **CLOSED** | `0001:134`（S5/S4；SPEC-0002 S9）；`0001:621`/`0001:633`（S4）；§16 V15-2 | — |
| **M-2** 组持久化 `CANCELLED` 写者 | **CLOSED** | `0001:115`（S3 写者限定整单取消；退款只改 S5）；`0001:385-388`（持久化 vs 读模型派生）；`0001:619`；§16 V15-6 | 无 |
| **M-3** §7.1 S1 迁移集合 / 全部退款行 | **CLOSED** | `0001:112`（首组运单创建）；`0001:371`/`0001:377`（图/注）；`0001:411-419`（拆 `PAID`/`ALLOCATED` 两行）；§16 V15-5 | 无 |
| **M-4** 零元路径 + ADR §5.3 枚举 | **CLOSED（登记前置）** | §16 V15-3（覆盖正金额/TCC/`system_zero`/legacy；ADR §5.3 additive amendment 先行）；§16.2 `AC-50-FIX-01`（owner Developer，expiry 2026-12-31） | 无（前置缺陷已登记） |
| **M-5** EARS-3/INV-44 观测义务 | **CLOSED** | `0001:607-610`（Inventory/Fulfillment 已确认事实；规范判定、不要求 Order 观测）；`0001:426`（多组释放缺口）；§16.2 `INV-44-MULTI-01` | 无 |
| **M-6** 3 组订单边界 | **CLOSED** | `0001:595-598`（2 组/剩余组条件；3 组按剩余组聚合）；`0001:422`（§7.5 注 1 以 2 组为例） | 措辞见 **N-2** |
| **M-7** 术语越权（表列/函数名/`state`） | **CLOSED** | `0001:617`（RefundAdmission 业务事实）；`0001:622`（唯一载体，不写表列）；`0001:623`（`state=PENDING_REMOTE` 中间态）；§16 V15-6（`effectiveGroupStatus` 只作证据） | 无 |
| **M-8** 永久不收敛 liveness | **CLOSED（登记）** | `0001:630-632`（DLQ/租约/AC-61 重放 IF）；§16.2 `AC-27-LIVE-01`（owner ops+AfterSales，expiry 2026-12-31） | 无 |
| **M-9** commit/blob 证据绑定 | **DEFERRED（accepted）** | §16.1 M-9；§16.3（Lead 实现前 commit 或记录 blob；不得与 ADR r0.4 混同一 change_id）；本报告已绑定 base/head/blob `73271a6` | Lead 在实现前 commit；不影响本轮 spec 判定 |
| **T-1** §16 行号陈旧 | **CLOSED** | §16 改用章节/AC 引用（V15-2/V15-3 等）；代码行号仅作 §16 证据 | — |
| **T-2** SPEC-0002 v1.4 引用未列全 | **CLOSED** | `0001:865`（“全部 `v1.4` 引用，含 §依据、§11、§13/§14 等”） | — |
| **T-3** AC-25/26 仍 Gherkin | **DEFERRED（accepted）** | §16.2 `EARS-25/26-01`（owner PO，expiry 2026-12-31） | 不阻塞 v1.5 的 AC-27/27A |
| **T-4** §8 未新增异常分支 | **CLOSED** | `0001:434`（指引 §9 AC-27A/§16.2） | — |
| **T-5** §6.7 关闭前提措辞 | **CLOSED** | `0001:303`（“退款完成或 AC-27A 关闭谓词满足”） | — |

**计数**：CLOSED 15（B-1、B-2、B-3(契约)、M-1…M-8、T-1/T-2/T-4/T-5）；DEFERRED 2（M-9、T-3）；0 OPEN。R1 的 9 个 `[MAJOR]` 中 8 个关闭、1 个（M-9）按 §16.3 登记为交付前动作；5 个 `[TODO]` 中 4 个关闭（T-1/T-2/T-4/T-5）、1 个（T-3）登记延期。

---

## 2. 任务重点逐项回答

1. **AC-27A 谓词是否只含 Order 自有持久事实、不再要求观测运单？** **是。** `0001:616-625` 明确输入只有“组投影状态、许可、组退款准入结果（RefundAdmission，owner=Order）”；第 4/5/6 条与 IF 均不读运单；`0001:638-639` 重申 Order 不得制造/回写/反推运单。旧版“无运单行/告警字段”已从读模型删除（`0001:294` 归 AC-50 写路径）。
2. **`CONSUMED+无运单 → CONFLICT` 是否已归 AC-50 写路径且不进入关闭谓词？** **是。** `0001:294`、§16 V15-3、§16.2 `AC-50-FIX-01` 明确写路径返回 `REFUND_ADMISSION_CONFLICT`、不写 PRE、不据此关闭；AC-27A 第 6 条把 `state=PENDING_REMOTE`/结果缺失归“未关闭”（`0001:623`）；且 V15-3 增加了“仅整组金额分支、部分金额先 `PARTIAL_REFUND_REQUIRES_FULL_GROUP`”的条件。
3. **AC-27 EARS-2 是否补 `CANCELLED` 且与 AC-25/§7.1/TCC 共存？** **是。** `0001:599-603` 枚举含 `CANCELLED`；`0001:604-606` 新增未支付取消/AC-51/AC-65 共存 EARS；与 `0001:112`（S1 6 值）、`0001:129`（AC-25）、`0002:236`/`0002:377`（TCC 先 CAS `CANCELLED`）一致。
4. **`order_refund_admissions.result` 载体与 `AC-50-FIX-01`/`AC-27-NOWB-01` 前置缺陷的 owner/expiry/未修前不得 VERIFIED 是否可执行？** **基本可执行，N-3 两点需补**：载体已唯一化（`0001:622`/`0001:643`）；`AC-50-FIX-01` owner=Developer（先 ADR amendment）、expiry=2026-12-31；`AC-27-NOWB-01`/`AC-27-LIVE-01` owner/expiry 已列（§16.2）；“未关闭前不得 AC-VERIFIED”写入 `0001:866`。但 `AC-27-NOWB-01` 的终态 ①/② 仍有 S6 owner 与 `FAILED_PERMANENT` 对齐问题（N-3）。
5. **§16 整改矩阵是否逐条对应？** **是。** §16.1 覆盖 R1 的 B-1/B-2/B-3、M-1…M-9、T-1…T-5 共 17 项，每项给出处置与落点；§16.2 给出 5 个前置缺陷/残余风险的 owner/expiry；§16.3 给出交付绑定与 SPEC-0002 同步；§16.4 给出下游影响。B-3/M-9/T-3 未被冒称为“已关闭”，而是“契约关闭/延期”，与文本一致；M-6 的“关闭”有 N-2 的措辞残留。

---

## 3. 整改引入的新问题（实现前条件）

### N-1 `[MAJOR]` — AC-27A:626-629 的迟到 `SHIPPED/RECEIVED` 分支与 AC-50 同 `refund_id` 幂等/INV-45 终态的关系未定义

- **证据**：`0001:626-629`（已关闭组随后出现 `SHIPPED/RECEIVED` → 读模型可转未关闭 + 告警，“退款准入按 AC-50 重新收敛为 `POST_SHIP_ALLOWED`”；“重复/乱序/未证实事件不得把已关闭组重新打开”）；`0002:227`（同一 `refund_id` 返回已持久化同一结果）；`0002:129`（INV-45 permit 终态不可恢复）；`0002:225`（已离仓 → POST）。
- **问题**：若该组已有 `PRE` 准入且已打款，“重新收敛为 POST”会被读成改写 terminal admission（违 AC-50:227/INV-45）；若要求新 `refund_id`，文本未写。另“证实/未证实事件”没有判定依据（S3 组投影只有状态，无事件 id/序号）。
- **改法**：在 AC-27A 或 §16 V15-2 补一句：“不得改写已持久化的 `PRE`/许可；已打款按已离仓售后/对账处理，未打款按**新 `refund_id`** 重新准入 `POST_SHIP_ALLOWED` 并保留原 PRE 审计；‘证实’仅指该组 S3 投影收到 `shipment.shipped/received`，重复事件幂等，无法判定时转 `AC-27-NOWB-01` 人工队列。”该条属异常分支，不影响第 1-6 条主谓词。

### N-2 `[MAJOR]` — AC-27:595 的“其余组均已关闭/已收货”条件对 3 组订单给出错误期望

- **证据**：`0001:595-598`（“仅当除 G1 外其余组均已关闭/已收货 … 仅剩 G1 且 G1 已发货、尚未收货时，`shipment_status` SHALL 为 `SHIPPED`”）；`0001:146`（INV-19 允许 3 组）；`internal/services/order/settlement.go:90-125`（其余组 RECEIVED 时返回 `PARTIALLY_RECEIVED`）。
- **问题**：若另一组已收货但未关闭，它仍在“剩余组”内，投影是 `PARTIALLY_RECEIVED` 而非 `SHIPPED`；“已收货”与“仅剩 G1”自相矛盾，M-6 的 3 组修正被这条措辞抵消。
- **改法**：把“除 G1 外其余组均已关闭/已收货”改为“除 G1 外其余组均已关闭（无其他未关闭组）”；或写成“无其他未关闭组时 …”。3 组示例保留 `PARTIALLY_SHIPPED`。属一行措辞修正，不改业务语义。

### N-3 `[MAJOR]` — `AC-27-NOWB-01` 终态 ①/② 的 S6 owner 与 `FAILED_PERMANENT` 对齐

- **证据**：`0001:857`（终态 ①“补齐运单取消证据→按 AC-50 合法分支重放→PRE→组关闭”；②“退款申请转 `FAILED_PERMANENT`/人工队列”；owner“Order 服务 owner（数据核对）+ ops/AfterSales”）；`0001:638-639`（Order 不得制造/写运单事实）；`0002:228`（准入未成功“申请保持可重试”）；`0002:311`（`FAILED_PERMANENT` 属 AC-62 退款执行/人工放弃）；`migrations/order/0017_refund_batch_ledger.sql:68-69`（`FAILED_PERMANENT` 是 application state）。
- **问题**：①“补齐运单取消证据”需要 S6（Fulfillment）侧的数据修复/核验，但 owner 未列 Fulfillment；Order 不得写运单，按现 owner 不可执行。②把 admission 冲突的申请转 `FAILED_PERMANENT` 与 AC-50:228“申请保持可重试”不一致，也未在 SPEC-0002/AC-62 的触发中授权。
- **改法**：owner 补 `Fulfillment`（S6 证据/数据修复）或明确“ops 凭 Fulfillment 授权核对，Order 不写运单”；②改为“申请保持可重试并由人工队列持有；仅在人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径”，或明确该终态是 v1.5 新增授权并请 SPEC-0002 owner 确认。

### N-4 `[TODO]` — `AC-27-NOWB-01` 的 owner 名称“Order 服务 owner”不是仓库 canonical 角色

- **证据**：`0001:857`（owner “Order 服务 owner（数据核对）+ ops/AfterSales”）；AGENTS §1 的角色矩阵是 PO/QC/Architect/Developer/Reviewer/Lead/ops。
- **改法**：改为 `Developer（Order 数据核对）/ops（重放、跨服务数据）/Fulfillment（S6 证据）`；三个前置缺陷的 expiry（2026-12-31）与 WIP waiver 到期同日，Lead 应在台账注明“若延期需重新裁决”。

---

## 4. 必答崩溃窗口（本 delta 范围）

| # | 窗口 | R2 结论 / 依据 | 缺口 |
|---|---|---|---|
| 1 | 本地提交后崩溃 | 关闭谓词纯读 → **N/A**；写路径修复（`AC-50-FIX-01`）为 Tx1 `PENDING_REMOTE`，崩溃后 AC-27A 第 6 条不关闭；恢复 owner = 发起方 AfterSales durable task（`aftersales/workers.go:390-404`）。 | v1.5 未在 AC 中写 owner，§16 `AC-27-LIVE-01` 已补 ops/AfterSales；不新增 blocker。 |
| 2 | 跨服务 RPC 成功后崩溃 | 运单 `CANCELLED` 已提交、Tx2 未提交 → `PENDING_REMOTE`，不关闭；按 `refund_id` 幂等重试；修复后按实际行状态重判（§16 `AC-50-FIX-01`“修复后按 AC-50 合法分支重放”）。 | 未明确“不得复用首次 reply”；N-1 建议补。 |
| 3 | 消息重复/乱序 | 重复 `aftersale.completed`/shipment 事件：`refund_state`/组 CAS 幂等；旧版“PRE 后迟到 shipped 重开”已由 AC-27A:626-629 定义（先离仓守卫、告警、重新收敛）。 | “证实/未证实”判定见 N-1。 |
| 4 | 取消/确认竞态 | 仍由 AC-48/AC-50 裁决（运单行 CAS，唯一胜者）；关闭谓词纯读，不新增资金/库存副作用；`CONSUMED+无运单` 归写路径 CONFLICT。 | 无新增。 |
| 5 | 外部重放缺失 | 谓词本身 N/A；`PENDING_REMOTE` 永久不收敛已由 AC-27A:630-632 + `AC-27-LIVE-01` 覆盖（保持未关闭、AC-61 人工重放、owner/expiry）。 | 终态转人工队列的细节见 N-3。 |

---

## 5. READY 的边界与下游

- **READY 的含义**：R1 的 3 个 BLOCKER 已关闭，17 项清单已关闭或按 §16.1 登记延期；本报告不把 v1.5 标为 AC-VERIFIED/Release PASS，也不批准 merge。
- **实现前条件**：N-1（AC-27A 迟到分支不得改写 PRE/许可，明确新 `refund_id` 或人工队列）、N-2（AC-27 EARS-1 的“已收货”措辞）、N-3（`AC-27-NOWB-01` 的 S6 owner/`FAILED_PERMANENT` 对齐）由 PO（必要时 Architect/ops）闭合；N-4 `[TODO]` 可随同修正。
- **交付前动作（不阻塞 spec 判定）**：M-9/§16.3——Lead 在实现前单独 commit SPEC v1.5 或记录 blob `73271a6`，不得与 ADR r0.4 r3 混同一 `change_id`；Reviewer 在 `ac-coverage.tsv` 分别登记 AC-27/AC-27A 行（AC-REF ≠ AC-VERIFIED）；`AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` 未关闭前不得声称 AC-27 EARS-1/EARS-2 收敛。
- **下游**：Architect 需先对 ADR Rev 2.3 §5.3 `CancelIfCreatedReply` 做 additive amendment（区分“无运单”），并在 ADR r03 撤销/改写 R-2、按 AC-27A 对齐 R-1；Developer 的 AC-50 修正覆盖正金额/TCC/`system_zero`/legacy，需 Lead 拆独立 `change_id` 走 L2；SPEC-0002 owner 在下次升版同步全部 `v1.4` 引用。

## 6. 审查边界、证据限制与只读声明

- **delta 证据限制**：上一被审 blob `efad166…` 不在 object DB（`git cat-file -t` 失败），无法字节级 diff；本报告用 R1 报告引文 + §16.1 矩阵 + 当前文本核读逐条判定，**不重审未改动全文**，不对未核对段落作 PASS 声明。
- **未执行**：`go test`、`-race`、真实 MySQL、迁移、故障注入；代码/迁移/SPEC 引用是只读事实，不构成执行证据；`CONSUMED+PRE`/`CONSUMED+无运单` 的生产分布未统计，标 `[不确定]`。
- **未覆盖**：R0.4 实现改动（`cmd/fulfillment/**`、`internal/platform/config/**`、`internal/services/fulfillment/**`、`migrations/order/0022_r04_checks.sql`、`deploy/**`、`scripts/**`、`docs/ops.md`）、ADR r0.4 r3 正文、SPEC-0002 正文修订、`docs/lead/**` 台账；这些属其他 change，不在本 R2 范围。
- **只读声明**：本报告只新建 `docs/collab/qc-reports/QC-PAY-SETTLE-01-SPEC-0001-V15-R2.md`；未改 SPEC/ADR/代码/迁移/台账；未 commit；不批准 merge、Release 或 AC-VERIFIED。
- **结论**：**READY**——R1 的 B-1/B-2/B-3 与 17 项清单已关闭/按 §16.1 延期；v1.5 可在闭合 N-1…N-3（及 N-4）后交人类批准/实现；在此之前仍不得 AC-VERIFIED。

---

## R2a 确认（2026-11-26，极简 delta；blob `d08bf87b063ad624c0f43846e57478d772488315`）

> 范围：只核对 R2 的 N-1…N-4 是否按其建议关闭，不重审全文；base/head 仍为 `110a0ff…`/`c432ed6…`（SPEC 未 commit）。只读核对当前工作树。

| N | 状态 | 证据（当前 SPEC） | 确认 |
|---|---|---|---|
| **N-1** | **CLOSED** | `0001:626-631`：已关闭组迟到 `SHIPPED/RECEIVED` 时，先按第 3 条离仓守卫；“证实”仅指 S3 收到 `shipment.shipped/received`；**不得改写已持久化 PRE/许可终态**；已打款按已离仓售后/对账；未打款按**新 `refund_id`** 重新准入 `POST_SHIP_ALLOWED` 并保留原 PRE 审计；重复幂等；无法判定转 `AC-27-NOWB-01` | 建议全部落点 |
| **N-2** | **CLOSED** | `0001:595`：“仅当除 G1 外其余组**均已关闭（无其他未关闭组**；例如 2 组订单）”；3 组示例保留 `PARTIALLY_SHIPPED`（`:597-598`） | “已收货”措辞已删除 |
| **N-3** | **CLOSED** | `0001:863`：终态①由 **Fulfillment** 提供/修复 S6 运单取消证据、ops 凭 Fulfillment 授权核对（Order 不写运单）→按 AC-50 重放→PRE；终态②申请**保持可重试**并由人工队列持有，仅人工放弃时按 AC-62 记 `FAILED_PERMANENT` 并取得 SPEC-0002 owner 书面口径；owner=Developer/ops/Fulfillment | S6 owner + `FAILED_PERMANENT` 已对齐 |
| **N-4** | **CLOSED** | `0001:863` owner 已改为 canonical `Developer/ops/Fulfillment`；`:868` 注“expiry 2026-12-31 与 WIP waiver 同日，延期需重新裁决” | canonical + expiry 注记 |

- **新 blocker：无。**
- 一个非阻塞 `[TODO]`（供 ADR/实现，不阻断本确认）：N-1 的“新 `refund_id`”路径未写原退款申请/commitment 的处置；建议 Architect 在 ADR 明确“先按 AC-62 合法触发（申请 `REJECTED/CANCELLED` 或人工授权）终止/释放原申请，再新建 `refund_id`”，避免同一组额度双重承诺（INV-24/AC-64）。
- **R2a 结论：CONFIRMED**（N-1…N-4 关闭；无新 blocker；SPEC 仍未 commit，交付绑定见 §16.3/M-9）。
