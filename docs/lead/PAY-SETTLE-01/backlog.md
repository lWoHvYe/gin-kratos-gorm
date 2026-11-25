# PAY-SETTLE-01 合并后 backlog / 搁置项清单

| 项 | 值 |
|---|---|
| 基线 | `main` `db5602f`；SPEC-0002 **v1.10** / ADR-0007 **Rev 2.3**；P0 WIP merge `d7c2f30` |
| 日期 | 2026-11-25 |
| 状态 | P0 主链路已 merge，**不是 Release PASS**；AC-39…68 仍 DEFERRED |
| 用途 | 区分“P0 收尾 / 同 Plan 未做 / 因主任务搁置 / 已撤回”四类，防止再次混做 |

## 1. P0 收尾（Release 前必须清）
> **2026-11-25 其他 agent 追溯复核（D-LC-5）**：Phase 0 exit 未被支持；
> T0.7 五方签署、C-05 真实迁移、`chk_switch_settled_expiry`、fail-closed
> config、C-04 release RPC 均有缺口。Phase 0-R 是 **P0 收口/Release 前置**；
> 当前 merge 只是 WIP，不是 P0 closure。详见 `tasks.md` Phase 0-R。

> **2026-11-26 状态**：R0.4 Addendum 已 ACCEPTED（人类 2026-11-26）；分支 A 修复
> `aec9ac8` + `4549ccb`，Reviewer **R0.4e-R2 PASS（scoped）**（F-1/F-2 CLOSED）；
> R0.3-R2 **PASS（scoped）**，AC-27 仍 **AC-REF**（AC-27A 第 5/6 条未实现，
> `AC-50-FIX-01`/`AC-27-NOWB-01`/`AC-27-LIVE-01` open）；**AC-50-FIX-01 的 ADR
> amendment 被 QC BLOCKED（R-2a 与 accepted ADR §5.2.2 冲突）**，需人类/Architect
> 裁决后再动；SPEC-0001 v1.5 已批准。**Phase 0-R 未闭合**，无 P0
> closure/Release/merge。


| ID | 内容 | 当前状态 / 缺口 | Owner |
|---|---|---|---|
| **P0-0** | **Phase 0-R 追溯整改（最高优先）** | R0.1 真实迁移 `order/0017-0020`+`fulfillment/0010`、`chk_switch_settled_expiry`、0018 重入/部分失败；R0.2 C-01..C-05 日期化 Reviewer verdict；R0.3 AC-27/回归/INV-45..50 + 12 orphan；R0.4 config fail-closed、C-04 release RPC、group_no/24h CHECK；R0.5 evidence-matrix 到 SPEC v1.10 | Developer + Reviewer + Architect + Lead/PO |
| P0-1 | **T1.7 竞态/故障测试**：Σ 不等、REFUNDING vs Ship、崩溃续跑、部分失败、无组 G1、已发货组 | 未开始 | Developer + Reviewer |
| P0-2 | **T5 证据矩阵**：AC-39…68、`evidence_path`、真实迁移、`-race`、故障注入、五方终态 | 未开始；台账全部 DEFERRED | Reviewer + Developer |
| P0-3 | **T1.4a 独立 scoped 裁决 / T-002 关闭** | T1.4a 被 T1.4b2 测试覆盖，但没有独立 Reviewer PASS；T-002 仍 OPEN | Reviewer |
| P0-4 | **任务/台账同步**：`tasks.md` 仍把 T1.2/T1.4/T1.5/T1.6 标未完成；T1.5 已 re-scope，T1.6 已 PASS | 文档漂移 | Lead |
| P0-5 | **多组订单 physical release** | P0 明确取舍：不释放，库存保持 reserved，P1/人工 | 后续任务 |
| P0-6 | **`inventory.confirm.requested` 的 `warehouse_id`** | paid-confirm 路径仍可能回退 W1；不阻塞 T1.6 | Developer |
| P0-7 | **NATS broker E2E / DLQ / 重投**；`payment_cancel_cas_lost_total` counter 值测试 | 未跑/未测；T5.2/T5.3 | Developer + Reviewer |
| P0-8 | **C-05 / 0012 真实迁移演练** | C-05 已合并；T5.4 仍要求真实迁移（非内联 DDL） | Developer + Reviewer |

**AC 现状**：AC-39…68 全部 `DEFERRED`（`ac-coverage.tsv`）。已有 scoped Reviewer PASS 不等于 AC-VERIFIED；AC-40（零元端到端）、AC-51/65（TCC/退款）等仍需 T5 证据。

## 2. 同 Plan、但尚未开始（P1/P2）

| ID | 内容 | 依赖 / 说明 |
|---|---|---|
| P1-T2 | 历史 `PAID` 解析/回填/人工队列：`ListOrderIntents` 唯一候选、`legacy_payment_resolution` SLA、`refund_seq`、回填 dry-run | T2.1–T2.7；先修生产者再回填 |
| P1-T3 | 行级/责任运费/commitment release：行层 INV-24、`UNDETERMINED/168h`、`RESERVED→RELEASED`、责任唯一 owner、AC-14/31/32 回归 | T3.1–T3.7；依赖 T1/T0.7 |
| P1-T4 | fail-closed + `order.settled` shadow：客户端构造期必填、启动 fail-fast、shadow 默认 paid、`AuthorizeShipment` 唯一权威 | T4.1–T4.6；依赖 T1/T3；不得切默认 |

## 3. 因 PAY-SETTLE-01 主任务而搁置

| ID | 内容 | 为什么搁置 / 当前状态 | 下一步 |
|---|---|---|---|
| S-0012 | **after-sales `0012` 长期拆分链** | 2026-11-23 决定 long-term split chain；dev/UAT 只到 `aftersales/0011`，无 0012 账本；主任务期间未动 | Architect 签拆分链 → expand/backfill/unique → 一次性库演练；不做操作员桥接 |
| S-P3 | **P3 产品/AI**：AI Turn/Action、quantity split、warehouse cost allocator、carrier、multi-warehouse seckill | P3-0 契约已完成；P3 实现被产品/AI Spec+ADR 阻塞；主任务期间未启动 | 先出 P3 Spec+ADR，不写代码 |
| S-AI6 | AI Phase 6：RAG cache/rerank 质量比较 | 版本化 RAG 已 dev；cache/rerank 质量比较仍薄 | 独立评估，不与 P0 混做 |
| S-AI8 | AI Phase 8：跨用户/no-memory benchmark | 现有 benchmark 仍单一 prompt family | 增加回归集 |
| S-AI9 | AI Phase 9：取消/退款异常解释 | 未实现 | 运营 copilot 后续 |
| S-H3 | H3：payment/order Developer deputy | Director 是 integration owner；deputy **TBD** | 人类指定；不阻塞代码 |
| S-H5 | H5：Mesh live UAT；生产 `ai.enabled=false` | 静态收窄已完成；live UAT 未跑；生产默认 false | 集群窗口跑 `check-ai-mesh.sh` 后再谈 AI 上线 |
| S-PROD | 生产备份/恢复、A/B/C 回滚，生产 split 开启 | 无大规模生产；生产 split 保持 off；仅 C fixture/UAT 演练 | 生产 rollout 前做 |
| S-CI | 真实 GitHub Actions / 发布流水线 | 未执行；门禁脚本已退役，G-1…G-4 仅人工清单 | 发布前跑真实 CI；不以“机器门禁通过”作证据 |
| S-P217 | P2#17 RAG cache/rerank 质量 | partial | 同上 |
| S-PUSH | `main` 远端状态/推送 | 本地 `main` 47f460b；origin/main 指向 merge d7c2f30；文档提交 1 个 ahead | 需要发布时再 push；当前未 push |

## 4. 已撤回 / 不作为 backlog

- **GOV-COST-01 / SPEC-0003 / ADR-0008 / Amendment 1**：2026-11-25 整体撤回；门禁脚本/CI 删除；G-1…G-4 只保留人工检查清单。
- **machine `make gates*` / `collab_gates.py`**：已删除，不得再要求运行。
- **T1.4b2 group-scoped reservation / C-05 Inventory 迁移 / LEGACY_SHARED / 生产 backfill**：已 withdrawn；P0 采用单组 release。
- **多组库存立即释放**：P0 明确 deferred；不要再以“补个 scan/last-active”方式加回。
- **T1.5 consumer observe-only**：当前保留为 cancel-after-confirm 载体；drain 前不切。

## 5. 建议顺序

1. **P0-4** 先修 tasks/台账漂移（纯文档，1 轮）。
2. **P0-1 + P0-2 + P0-3**：T1.7、T5、T1.4a/T-002；这是 Release 前的主线。
3. **P0-6/P0-8/P0-7**：confirm warehouse、真实迁移、NATS/DLQ。
4. **P1-T2 → P1-T3 → P1-T4**：历史 PAID → 责任/commitment → fail-closed/shadow。
5. **S-0012**：与 T3 解耦，单独 Architect/迁移演练。
6. **S-P3 / AI**：等 P0/P1 收口后另立 Spec/ADR，不与退款链路混做。
7. **Release**：AC-39…68 验证/显式延期、五方终态、真实迁移/CI、人类 `RELEASE_APPROVED`；否则只叫 merge/PASS scoped。

## 6. 当前不能说的话

- 不能说“PAY-SETTLE-01 完成”：P0 主链路 merge，但 T1.7/T5/T2/T3/T4 未完成。
- 不能说“AC-39…68 已验证”：全部 DEFERRED。
- 不能说“Release PASS”：无 Release 证据。
- 不能说“多组退款库存已释放”：P0 已知 deferred。
