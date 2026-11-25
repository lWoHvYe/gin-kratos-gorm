# Reviewer 工作记忆（长期）

> 所有者：Reviewer。其他角色只读。
> 用途：跨会话保留**红队题库、常用命令、易漂移点、角色边界与决策索引**；不记录运行状态。
> 规则：这里不放业务规则、不放设计结论、不写实现细节；业务以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准，代码与测试以 `internal/**` 为准。
> 只收跨变更仍成立的稳定经验；运行状态见 `docs/review-tracking.md` / `ac-coverage.tsv`；协作闸门已于 2026-11-25 退役，pair 线程已退役（2026-11-25）。

## 1. 角色与硬边界

- **输入**：`Spec.md` / `docs/specs/**`、`docs/adr/**`、代码、测试日志、偏差登记、Lead 路由的跨角色问题。
- **输出**：`PASS / FAIL` + 按 AC 逐条核对 + 红队问题；结论落在 `docs/review-tracking.md`。`docs/collab/developer-reviewer/threads/T-NNN.md` 为历史存档（pair 线程 2026-11-25 退役），不再新开。
- **不写代码**、**不改** `docs/specs/**`、`docs/adr/**`、`docs/implementation/**`、`internal/**`；只提议，由 Developer 实施。
- 可以维护：`docs/reviewer/**`（本文件）、`docs/collab/ac-baseline.txt`、`docs/collab/ac-coverage.tsv`；`docs/collab/developer-reviewer/**` 仅存档只读。
- 不改对方已写内容：Developer 的偏差登记、代码、历史 `T-NNN` 回复段只读；要改让对方改。跨角色交互统一提请 Lead 路由至 owner（pair 线程 2026-11-25 退役）。
- 不微观完美主义阻断交付；但以下必须阻断：**自检清单缺失、未覆盖 AC、核心资损/并发/状态机违约、未登记偏差、混提/越权**。

## 4. 红队题库（每轮至少抽 3 条追问）

1. **并发/幂等**：同一行两笔售后、同一订单重复支付、同一运单重复/乱序轨迹、同一缺货事件重复投递，谁保证只发生一次？
2. **额度**：行剩余可退额、订单 captured 剩余、运费累计封顶，三层是否都扣了 pending/在途；并发下是否先锁后算、锁跨 Payment 调用？运费 hold 的结算是否绑定商品退款完成，而不是只看 168h 到期？
3. **时序**：`occurred_at` 从请求体直取是否可信；`delivered_at` 与 job `due_at` 是否可能不一致；60s grace 是否真能覆盖同刻申诉。
4. **状态机**：有没有把退款态写进 `orders.status`；组 SHIPPED/RECEIVED 是否只由事件驱动；客服是否还能直接改签收。
5. **Fail-Closed**：配置缺失、下游超时、outbox 失败时是阻断还是静默放行；`try-catch` 是否吞掉资金/库存异常。
6. **可验证性**：测试是否只断言“接口返回成功”，而没有断言最终账、最终状态、累计上限；Playwright/同刻竞态是否被“后端字段测试”冒名顶替。
7. **简化与范围**：本 FAIL 是否有语义更简的等价方案？被砍掉的范围是否需要人类显式接受？

## 5. 常用命令

```bash
# AC 引用覆盖（CONVENTIONS §4.1）
for n in $(seq -w 1 38); do
  c=$(grep -rl "AC-$n" --include='*_test.go' . | wc -l | tr -d ' ')
  [ "$c" = "0" ] && echo "MISSING AC-$n"
done

# 协作闸门已于 2026-11-25 退役；不再运行 make gates*
# 合并前以 make test / make verify（不含 gates）为准

# 目标测试（示例）
export PATH="$PWD/.tmp/bin:$PATH"
export GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp
export GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath
export GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_review?parseTime=true&loc=UTC' \
  go test ./internal/services/aftersales -run 'TestAC' -count=1
git log --oneline -8 && git status --short
```

## 6. 易漂移点（审查前先看）

1. `38/38` 只证明测试文件里出现过 `AC-NN`，**不证明** AC 语义和 ADR §14 指定的验证方式。
2. `TestAC<NN>` 薄 wrapper 共用同一 body（AC-21/37、AC-31/32/33）时，要逐条看断言是否覆盖各自 AC。
3. 协作闸门已退役，不存在机器扫描范围；合并范围改由人工 `git log`/`git diff` 核对。
4. 台账 `ac-coverage.tsv` 归 Reviewer；空台账 + 测试引用 = 虚假 38/38。
5. 签收链路：`delivered_at` 是事实时刻，`occurred_at` 是事件声明的时刻；job 的 `due_at` 不得被后来的重复事件用 `LEAST` 提前。
6. 资金链路：`after_sale_line_ledger.pending_*`、`after_sale_amount_ledger.consumed`、Payment captured 三层缺一不可；`recordLineRefundTx` 没有 DB 上限约束，别只看代码。
7. 所有“开关”都要在 `internal/platform/config/config.go` 找到 env 解析；只在文档里写开关等于没有。
8. 偏差登记只认 `docs/implementation/**`；代码里的 `// TODO` 不能替代。
9. **运费 hold 结算**：不能只按 `hold.status=HELD && deadline<=now` 扫（`responsibility.go:312-317`）；必须绑定 `after_sales.status=REFUNDED`（或商品退款完成），否则拒绝单/未退款单会白退运费。`refundFreightHold` 的 breakdown 用 upsert（`:220-230`），`freightCommitted` 排除 REJECTED/CANCELLED（`:349-355`）；hold 创建时 effective 只含商品（`service.go:283-286`），退款时被 effective 截断（`workers.go:414-416`），保证 hold 与 goods refund 不重复付同一笔运费。
10. 开关默认 false **不是免责**：功能开关打开后的资损路径按“已交付”审查；`FREIGHT_RESPONSIBILITY_ENABLED` 仍按 ADR §13.1 分阶段灰度，不因 PASS 自动打开。
11. `[TODO]` 长期停在 `REQUESTED/APPROVED` 的售后 hold 不会被 worker 结算，却仍被 `freightCommitted` 计入；到期自动 WAIVED 或排除未批准请求的清理项属已知缺口，跟进状态见 `docs/review-tracking.md`。

## 7. 新会话开场动作

1. 读 `AGENTS.md`、`docs/collab/CONVENTIONS.md`、当前 Spec 与相关 ADR、本文件；运行状态与待闭环项以 `docs/review-tracking.md` 为准。
2. `git status --short` + `git log --oneline -8`：确认是否有其他 Agent 的在途改动；有则不动并提请 Lead。
3. 先处理 `docs/review-tracking.md` 中 `OPEN` 且 `BLOCKING` 的条目；Developer 未回写前不关闭、不降级为 PASS。跨角色问题提请 Lead 路由至 owner。

## 8. 目录约定（2026-11-22 人类指挥官约定）

- 记忆区：`docs/reviewer/**`（唯一作者 Reviewer）。
- 审查渠道：`docs/collab/developer-reviewer/**`（README + threads.md + threads/T-NNN.md）自 2026-11-25 起为**历史存档**，只读；新结论落 `docs/review-tracking.md`。
- 提交 scope：`docs(reviewer)`；trailer 用 `Role: Reviewer`；pair 线程退役后不再新增 `Thread: developer-reviewer/T-NNN`。
- 协作闸门脚本已删除（2026-11-25）；不再有 `MANAGED` 登记或脚本复核问题。

## 9. 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/**` | 业务结论，唯一事实源；Reviewer 只读 |
| `docs/adr/**` | 设计结论，唯一事实源；Reviewer 只读 |
| `docs/implementation/*-deviation-register.md` | 实现偏差（owner + expiry）；Reviewer 核验 |
| `docs/collab/developer-reviewer/threads/` | 审查过程与红队问题（历史存档，2026-11-25 退役）；**结论以代码+日志+Spec/ADR 为准** |
| `docs/collab/ac-baseline.txt` / `ac-coverage.tsv` | 人工 AC 台账；Reviewer 维护（无机器校验） |
