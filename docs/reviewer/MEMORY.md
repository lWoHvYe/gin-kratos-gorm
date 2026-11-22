# Reviewer 工作记忆（长期）

> 所有者：Reviewer。其他角色只读。
> 用途：跨会话保留**审查闸门状态、待闭环阻断项、红队问题、命令与易漂移点**。
> 规则：这里不放业务规则、不放设计结论、不写实现细节；业务以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准，代码与测试以 `internal/**` 为准。

## 1. 角色与硬边界

- **输入**：`Spec.md` / `docs/specs/**`、`docs/adr/**`、代码、测试日志、偏差登记、协作线程。
- **输出**：`PASS / FAIL` + 按 AC 逐条核对 + 红队问题；结论落在 `docs/collab/developer-reviewer/threads/T-NNN.md`。
- **不写代码**、**不改** `docs/specs/**`、`docs/adr/**`、`docs/implementation/**`、`internal/**`；只提议，由 Developer 实施。
- 可以维护：`docs/reviewer/**`（本文件）、`docs/collab/developer-reviewer/**`（审查线程）、`docs/collab/ac-baseline.txt`、`docs/collab/ac-coverage.tsv`。
- 不改对方已写内容：Developer 的偏差登记、代码、T-NNN 回复段只读；要改让对方改。
- 不微观完美主义阻断交付；但以下必须阻断：**自检清单缺失、未覆盖 AC、核心资损/并发/状态机违约、未登记偏差、混提/越权**。

## 2. 当前基线（2026-11-22 复核通过）

| 项 | 值 |
|---|---|
| 审查基线 | `f162ecf`（P0-7 修复；Developer 回复 `5b9e18e`；初查 `8ea9236`） |
| Spec | SPEC-0001 **v1.2 approved** |
| ADR | ADR-0005 **v1.2**、ADR-0006 accepted |
| 实施线程 | `architect-developer/T-001`、`T-002` 均 **CLOSED**（`c085006` 追认 scope） |
| 审查线程 | `docs/collab/developer-reviewer/threads/T-001.md` **CLOSED / PASS**（`closed_by=f162ecf`） |
| 审查结论 | **PASS**：P0-1…P0-7 全关闭；P0-6 剩余项为有 owner/expiry 的到期项 |
| 闸门命令 | `make gates-strict` → 38/38、`G-1=0 G-2=0 G-3=0 exit=0`；默认范围只到 `HEAD^..HEAD` |
| AC 台账 | `ac-coverage.tsv` 已有 AC-22/28/29/30/35/37 的 owner/expiry/thread，Reviewer 复核接受为到期项 |

> `FREIGHT_RESPONSIBILITY_ENABLED` 默认 false；P0-7 已修复并复跑通过，按 ADR §13.1 分阶段灰度，不在本审查中自动打开。

## 3. 审查项状态（下轮开场先看）

| ID | 事项 | 契约 | 状态 |
|---|---|---|---|
| P0-1 | 自检清单三问 | `AGENTS.md` §1.3 | **CLOSED**（T-001 §7.2） |
| P0-2 | 行级 `pending_*` 在途占用 | ADR-0005 §6.4 / §19.1#12 | **CLOSED**（四路径 + 并发测试） |
| P0-3 | `FREIGHT_RESPONSIBILITY_ENABLED` 开关 | ADR-0005 §13.1 | **CLOSED**（config + legacy 分支 + 测试） |
| P0-4 | 重复 `DELIVERED` 回拨 `due_at` | Spec AC-20/22/35 | **CLOSED**（锚定 `delivered_at` + 回归测试） |
| P0-5 | 历史混提 + 自改 CONVENTIONS/台账 | CONVENTIONS §1/§5.1/§5.2 | **CLOSED**（D.1 `[mixed]` + T-002 追认） |
| P0-6 | AC-14/22/28/29/35/37 深度、台账为空 | CONVENTIONS §4.1/§A.4、ADR §14 | **CONDITIONAL**（台账已登记；AC-22 worker/页面断言 `2026-12-15` 到期） |
| P0-7 | 运费 hold 结清未绑定售后状态/商品退款 | Spec 6.5.1/6.5.2、ADR §6.5/§18.4 | **CLOSED**（`f162ecf`；驳回 WAIVED、worker 只结算 REFUNDED、breakdown upsert、committed 排除驳回） |

## 4. 红队题库（每轮至少抽 3 条追问）

1. **并发/幂等**：同一行两笔售后、同一订单重复支付、同一运单重复/乱序轨迹、同一缺货事件重复投递，谁保证只发生一次？
2. **额度**：行剩余可退额、订单 captured 剩余、运费累计封顶，三层是否都扣了 pending/在途；并发下是否先锁后算、锁跨 Payment 调用？运费 hold 的结算是否绑定商品退款完成，而不是只看 168h 到期？
3. **时序**：`occurred_at` 从请求体直取是否可信；`delivered_at` 与 job `due_at` 是否可能不一致；60s grace 是否真能覆盖同刻申诉。
4. **状态机**：有没有把退款态写进 `orders.status`；组 SHIPPED/RECEIVED 是否只由事件驱动；客服是否还能直接改签收。
5. **Fail-Closed**：配置缺失、下游超时、outbox 失败时是阻断还是静默放行；`try-catch` 是否吞掉资金/库存异常。
6. **可验证性**：测试是否只断言“接口返回成功”，而没有断言最终账、最终状态、累计上限；Playwright/同刻竞态是否被“后端字段测试”冒名顶替。

## 5. 常用命令

```bash
# AC 引用覆盖（CONVENTIONS §4.1）
for n in $(seq -w 1 38); do
  c=$(grep -rl "AC-$n" --include='*_test.go' . | wc -l | tr -d ' ')
  [ "$c" = "0" ] && echo "MISSING AC-$n"
done

# 闸门
make gates-strict
RANGE=origin/main..HEAD make gates-strict   # 合并前扫全量，不要只看 HEAD

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
3. `gates-strict` 默认范围是单笔提交；Reviewer 需要显式用 `RANGE=...` 扫合并范围，否则历史混提看不见。
4. 台账 `ac-coverage.tsv` 归 Reviewer；空台账 + 测试引用 = 虚假 38/38。
5. 签收链路：`delivered_at` 是事实时刻，`occurred_at` 是事件声明的时刻；job 的 `due_at` 不得被后来的重复事件用 `LEAST` 提前。
6. 资金链路：`after_sale_line_ledger.pending_*`、`after_sale_amount_ledger.consumed`、Payment captured 三层缺一不可；`recordLineRefundTx` 没有 DB 上限约束，别只看代码。
7. 所有“开关”都要在 `internal/platform/config/config.go` 找到 env 解析；只在文档里写开关等于没有。
8. 偏差登记只认 `docs/implementation/**`；代码里的 `// TODO` 不能替代。
9. **运费 hold 结算**：不能只按 `hold.status=HELD && deadline<=now` 扫（`responsibility.go:312-317`）；必须绑定 `after_sales.status=REFUNDED`（或商品退款完成），否则拒绝单/未退款单会白退运费。`refundFreightHold` 的 breakdown 用 upsert（`:220-230`），`freightCommitted` 排除 REJECTED/CANCELLED（`:349-355`）；hold 创建时 effective 只含商品（`service.go:283-286`），退款时被 effective 截断（`workers.go:414-416`），保证 hold 与 goods refund 不重复付同一笔运费。
10. 开关默认 false **不是免责**：功能开关打开后的资损路径按“已交付”审查；P0-7 已修复，但 `FREIGHT_RESPONSIBILITY_ENABLED` 仍按 ADR §13.1 分阶段灰度，不因 PASS 自动打开。
11. `[TODO]` 长期停在 `REQUESTED/APPROVED` 的售后 hold 不会被 worker 结算，却仍被 `freightCommitted` 计入；到期自动 WAIVED 或排除未批准请求的清理项已交 Developer（不影响本轮 PASS）。

## 7. 新会话开场动作

1. 读 `AGENTS.md`、`docs/collab/CONVENTIONS.md`、当前 Spec、ADR-0005/0006、本文件、`docs/collab/developer-reviewer/threads.md`。
2. `git status --short` + `git log --oneline -8`：确认是否有其他 Agent 的在途改动（当前工作区 `prompt/*` 有未提交改动，非本人产物，不动）。
3. 先处理 `OPEN` 且 `BLOCKING` 的审查线程；Developer 未回写前不关闭、不降级为 PASS。
4. 每轮结束刷新本文件第 2、3 节和线程索引。

## 8. 目录约定（2026-11-22 人类指挥官约定）

- 记忆区：`docs/reviewer/**`（唯一作者 Reviewer）。
- 审查渠道：`docs/collab/developer-reviewer/**`（README + threads.md + threads/T-NNN.md）。
- 提交 scope：`docs(reviewer)`；线程与索引沿用 `docs(collab)`；trailer 用 `Thread: developer-reviewer/T-NNN`、`Role: Reviewer`。
- 闸门脚本 `scripts/collab_gates.py` 的 `MANAGED` 尚未登记 `docs/reviewer/`；该脚本改动需 Architect + Reviewer 双方复核（CONVENTIONS §A.6），当前记 `[TODO]`，不单方面改。

## 9. 与其他文档的关系

| 文档 | 角色 |
|---|---|
| `docs/specs/**` | 业务结论，唯一事实源；Reviewer 只读 |
| `docs/adr/**` | 设计结论，唯一事实源；Reviewer 只读 |
| `docs/implementation/*-deviation-register.md` | 实现偏差（owner + expiry）；Reviewer 核验 |
| `docs/collab/developer-reviewer/threads/` | 审查过程与红队问题；**结论以代码+日志+Spec/ADR 为准** |
| `docs/collab/ac-baseline.txt` / `ac-coverage.tsv` | 闸门台账；Reviewer 维护 |
