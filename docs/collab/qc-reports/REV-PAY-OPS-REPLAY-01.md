# REV-PAY-OPS-REPLAY-01 — OPS-P4 最终 scoped 复审 + AC-27 判定

| 项 | 值 |
|---|---|
| change_id / Parent | `PAY-OPS-REPLAY-01` / `PAY-SETTLE-01`（暂停） |
| Tier | **L2（确认）**：P0、退款/履约、跨服务恢复、人工运维；不降级 |
| review_scope | **OPS-P4**：runbook + drill 的最终 scoped 复审；D1–D11、授权/S6/审计/回滚/禁止项、AC-27 判定；不重开 T0–T8/R0.3-R1c |
| base | **`1e5e7a0`**（OPS-P4 human residual acceptance） |
| runbook/drill head | **`5ef1b357fe6d62fe7dea2c15980ab8149d537d33`** |
| selfcheck | **`42931a7`** |
| docs/QC head（复审绑定） | **`e3c68a2371cb40007ef26c3e08c0a6b2dc32fcbb`** |
| 契约输入 | ADR `0007-amd1-addendum-ops-manual.md` DRAFT 0.1；QC `MANUAL-R2` READY（0 BLOCKER/3 TODO）；SPEC-0001 v1.5 §16.2；SPEC-0002 AC-61/62；waiver C+A（`review-tracking.md:458-483`） |
| integration_owner | ops（执行/重放）/Developer（Order 数据）/AfterSales（task）/Fulfillment（S6 证据）；Reviewer 只读核验 |
| INV | **INV-45**（无 release/permit restore）、INV-44/50 边界、AC-27A/AC-61/62 不越权升级 |
| verification | §1：`./scripts/drill-ops-replay.sh` 在 HEAD `e3c68a2` 真实 MySQL 8.4.13 + NATS 2.10.29 运行 **EXIT=0 / 60 OK / 0 FAIL**；`migrations` diff 空；无生产代码 |
| 日期 / Actor / Reviewed-by | 2026-11-27 / `reviewer-r04` / `human (merge pending)` |
| release_status | **NOT RELEASE** |
| overall verdict | **PASS（OPS-P4 runbook/drill scoped）；AC-27 = AC-REF（不升级）**；D1/D5 缺真实 consumer/worker 收敛断言，见 §4 |

---

## 0. 结论

**OPS-P4 的 runbook 与 drill 本身 PASS（scoped）。** `drill-ops-replay.sh` 在精确 HEAD `e3c68a2`、真实 disposable MySQL **8.4.13** + NATS **2.10.29** 上运行 **60 OK / 0 FAIL / EXIT=0**；真实迁移、D1–D11、D5b/D10b 错组、D7/D11 再 Refund、D8 伪造/过期/同人、D9 并发、最终 0 duplicate refund/0 release/permit 不变均有断言；`migrations` diff 为空，`internal/**` 未改。

**但 AC-27 不能升级为 AC-VERIFIED。** selfcheck §3 明确承认：
- **D1** 不启动 Order `aftersale.completed` consumer，使用合成 DLQ wrapper；只验证 wrapper→`data` 字节、JetStream 去重与 DB 快照，**不验证真实 consumer 投影收敛**（`docs/implementation/PAY-OPS-REPLAY-01-selfcheck.md:56-57`）。
- **D5** 只验证 re-arm 两条 CAS；**不验证** worker/TCC re-drive 后的 PRE/POST（同上）——drill D5 输出 admission 仍 `PENDING_REMOTE`，没有 PRE/关闭。
- D7b 只验证“不自动重开”，不覆盖真实用户再 Refund；`after_sales`/Order consumer 未运行。

因此 AC-27 的终态收敛（`re-arm/DLQ → worker/consumer → PRE/关闭`）仍未被真实执行；runbook 可执行 ✅、drill 的 SQL/CAS 与拒绝路径 ✅，但不能冒充 AC-27 全链路验收。**AC-27 保持 AC-REF**；AC-27A 5/6 仍按 R0.3-R1c 的 PASS 证据，不因本 L2 升级；AC-61/62 仍 AC-REF；AC-72 不升级。

---

## 1. 实际执行（HEAD e3c68a2）

```bash
cd /Users/lWoHvYe/IdeaProjects/gin-kratos-gorm
./scripts/drill-ops-replay.sh     # EXIT=0
```

| 项 | 结果 |
|---|---|
| 绑定 HEAD / runbook-drill | **e3c68a2** / `5ef1b35`（runbook/drill 未变） |
| MySQL / NATS | **8.4.13** / **2.10.29**（disposable，真实 migrations） |
| D1–D11 | **60 OK / 0 FAIL** |
| D5b/D10b 错组 | PASS：`S6_GROUP_MISMATCH`，零写 |
| D7/D11 再 Refund | PASS：task/app `FAILED_PERMANENT`、不自动重开；TCC app FAILED |
| D8 伪造/过期/同人 | PASS：`REVIEWER_REQUIRED`/`AUTHORIZATION_EXPIRED`/`REVIEWER_AUTHORIZER_SAME`/`OWNER_APPROVAL_MISSING`/`AUTHORIZATION_ID_UNVERIFIED` |
| D9 并发 | PASS：两个 CAS 仅 1 胜，version 只 +1 |
| final invariants | PASS：0 release、permit `CONSUMED`、`refunded_cent=0`、每 refund 1 admission、TCC app 无重复 |
| `migrations` / 生产代码 | **空 / 无改动**（`git diff 5ef1b35..e3c68a2 -- internal migrations scripts docs/ops.md` 仅 docs/ADR/QC） |

原始日志：`.tmp/evidence-ops-replay/reviewer-drill-run.log`（本次审查生成，未入库）。

---

## 2. Runbook 可执行性与控制面

| 检查 | 结论 | 文件:行号 |
|---|---|---|
| 人类残余签署句（无技术不可抵赖；命名账号/双人/工单/DBA/Reviewer 补偿；不得声称 AC-62 技术闭环） | **PASS（文本）** | `docs/ops.md:941-1210`（runbook 首段）；waiver `docs/review-tracking.md:458-483` |
| 授权字段：authorization_id/op_id/action/identifiers/amount/三 admin/expiry≤24h/ticket=RVW/owner_approval | **PASS** | `docs/ops.md:957-978`（授权矩阵） |
| ≥¥1000（100000 cent）第二复核、reviewer≠authorizer、operator≠authorizer | **PASS** | runbook 授权矩阵；drill D8（`scripts/drill-ops-replay.sh:478-503`） |
| S6 仅 Fulfillment `order_no+group_no` 证据，Fulfillment authorizer 签字/归档；错组拒绝 | **PASS** | `docs/ops.md:1046-1096`；drill D5b/D10b |
| 审计先于 CAS：变更单/结构化 `review_rearm`/`after_sale_logs`/admission+batch version/时间戳/DBA log | **PASS** | `docs/ops.md:1057-1100`；drill D5/D7 |
| 失败/回滚：不自动反向改 CAS、不清 `last_error`、不删 admission、不恢复 permit/release | **PASS** | `docs/ops.md:1163-1179` |
| 禁止项：无 `cmd/ops`/`0024`/授权表/新 RPC/新迁移/自动 re-arm/release/close/伪造 S6/共享账号 | **PASS** | `docs/ops.md:1175-1179` |
| 越权/伪造窗口 | **残余（waiver 已接受）**：authorization_id/owner_approval 可工单自报、无密码学不可抵赖、S6 group 依赖 authorizer 签字；Reviewer 必须核对原件（`docs/ops.md:1151-1161`；waiver `:461-470`） | 不作为技术控制声称 |
| AC-27A/AC-61/62 范围 | **PASS（范围收窄）**：runbook 明示 waiver 仅 AC-27-NOWB/LIVE；AC-27A 不在 waiver；AC-61/62 不全量 | `QC-PAY-OPS-REPLAY-01-MANUAL-R2.md:32-40`；`docs/ops.md:941-1210` |

**Runbook 可执行：PASS（文本/操作层面）；伪造/越权窗口由 waiver 的命名账号+双人+工单+DBA 审计+Reviewer 补偿，不得声称技术不可抵赖。**

---

## 3. D1–D11 覆盖矩阵

| 演练 | drill 结果 | 是否足够 AC-27 |
|---|---|---|
| D1 DLQ 正常 replay | 提取原 bytes、JetStream 去重、DB 快照不变 | **不足**：未启动真实 consumer，无投影收敛 |
| D2 DLQ 缺失/超 7 天 | fail-closed | 足够（拒绝路径） |
| D3 task 租约丢失 | 行锁 CAS、canonical 保留、admission 不变 | **部分**：未启动 worker 自动恢复 |
| D4 FAILED_PERMANENT | 拒绝重放、无新 admission | 足够（拒绝路径） |
| D5 re-arm CANCELLED | admission/batch CAS、canonical 保留、permit 不变、0 release | **不足**：未运行 worker/TCC re-drive，未到 PRE/POST |
| D5b/D10b 错组 | `S6_GROUP_MISMATCH`，零写 | 足够（拒绝路径） |
| D6 SHIPPED→POST | 映射 POST，不写 PRE | **部分**：映射断言，非真实 admission |
| D7/D7b abandon | task FAILED、admission REVIEW、commitment RESERVED、permit/组不变、0 release | 足够（终态②人工路径）；用户再 Refund 仅“不自动重开” |
| D8 授权拒绝 | 缺 reviewer/过期/同人/owner/伪造均拒绝零写 | 足够（脚本化；原件核对为 Reviewer） |
| D9 并发 | 仅 1 CAS 成功 | 足够（CAS 层） |
| D11 TCC abandon | app FAILED、batch MANUAL_HOLD、canonical、0 release | 足够（终态② TCC） |
| final invariants | 0 duplicate/0 release/permit 不变 | 足够（不变量） |

---

## 4. AC-27 判定（AC-VERIFIED? **NO**）

| 项 | 判定 | 依据 |
|---|---|---|
| AC-27A 第 5/6 条 | **PASS（R0.3-R1c）** | `REV-PAY-SETTLE-01-R0.3-R1c.md`；`9a57d01`；不因本 L2 升级 |
| AC-27-NOWB-01 终态①（re-arm→PRE/组关闭） | **OPEN（未验证）** | D5 只做 CAS，admission 仍 `PENDING_REMOTE`；未运行 worker/TCC；selfcheck `:56` |
| AC-27-NOWB-01 终态②（abandon） | **PASS（runbook + D7/D11）** | D7/D11；admission REVIEW/commitment RESERVED/permit 原值/0 release |
| AC-27-LIVE-01（DLQ/租约→重放收敛） | **OPEN/BLOCKED（未验证）** | D1 合成 DLQ wrapper，未运行 `aftersale.completed` consumer；D3 未运行 worker；selfcheck `:56-57` |
| AC-27 整体 | **AC-REF（不升级）** | 终态①与 LIVE 收敛未真实执行；waiver 允许但未满足断言 |
| AC-27A 不在 waiver | **保持 R1c 证据，不升级** | QC MANUAL-R2 M4；waiver scope |
| AC-61/62 / AC-72 | **不升级** | AC-61/62 AC-REF；AC-72 ALERTS/ENTRY 仍 DEFERRED |

**最小剩余工作**（不新写业务代码；补真实执行/证据）：
1. 用**既有** Order `aftersale.completed` consumer 重放 D1 原事件，证明 `PENDING_REMOTE` 收敛或转 NOWB；记录 before/after 投影。
2. 用**既有** worker/TCC entry 执行 D5 re-arm 后的重驱动，证明 `PRE_SHIP_REVOKED`/组关闭（CANCELLED）或 POST（SHIPPED）；不得只停在 CAS。
3. 真实用户再 Refund 路径（无代码 guard）由工单/命名账号拦截；若无法演练，明确保留 waiver 残余，不得声称 AC-62 全量。
4. AC-27A 仍按 R0.3-R1c 的独立证据；不把本 waiver 扩到 AC-27A/AC-61/62。

Owner：Developer（Order consumer/worker）+ ops（执行/工单）+ AfterSales（task）；expiry 2026-12-31。

---

## 5. 台账

- `ac-coverage.tsv` AC-27：**AC-REF**；reason 更新为 OPS-P4 runbook/drill 60 OK 但 D1/D5 未跑真实 consumer/worker，NOWB/LIVE 收敛未验证；test_type `unit+MySQL+runbook+drill`；evidence 增加 `docs/ops.md:941-1210`、`scripts/drill-ops-replay.sh`。
- AC-27A：按 R0.3-R1c 证据，不因本 L2 升级；AC-61/62 仍 AC-REF；AC-72 不升级；未删/未重编号/未批量续期。
- `REVIEW-REQUIRED-ALERTS-01`/`ENTRY-01` 仍 DEFERRED；`PAY-SETTLE-01` 仍冻结。

---

## 6. INV-45 / 宪法 / Governance

| 项 | 结论 | 依据 |
|---|---|---|
| INV-45 | **PASS** | drill final permit `CONSUMED`、0 release；abandon 不 release/不恢复 permit |
| I.1/I.4 | **PASS** | 无生产代码；人工规程不夹带工具/迁移/RPC |
| I.2 Fail-Closed | **PASS** | D2/D4/D5b/D8/D10 拒绝零写；D9 单 CAS |
| I.3 契约优先 | **PASS** | SPEC v1.5 §16.2/AC-61/62；AC-27A 不越权 |
| II.1/II.3 | **PASS** | abandon 非 release；REVIEW 仍负债；重复 replay 幂等 |
| III.2/III.4 | **PASS** | 真实 MySQL/NATS；无新 schema/RPC/worker |
| IV.1/IV.2 | **PASS（有范围）** | 60 OK 真实日志；未跑 consumer/worker 已声明 |
| IV.4 | **PASS** | `NOT RELEASE`；不声称 AC-27A/AC-61/62 全量 |
| Workflow 9 / Governance | **PASS** | L2；waiver C+A 记录；人类 merge pending |

**`[ARCHITECTURAL_FLAW]`：无。** 当前是证据/验收范围问题，不是契约或实现缺陷；不得用局部代码补丁替代真实 consumer/worker 证据。

---

**Actor**：`reviewer-r04`（Reviewer）
**Reviewed-by**：`human (merge pending)`
**日期**：2026-11-27
**报告**：`docs/collab/qc-reports/REV-PAY-OPS-REPLAY-01.md`
