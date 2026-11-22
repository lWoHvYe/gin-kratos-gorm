# Developer 工作记忆（长期）

> 所有者：Developer。其他角色只读。
> 只放“下次开新会话必须记得”的角色边界、当前状态、易漂移点与命令；**不承载业务规则**。业务以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准。

## 1. 角色与硬边界

- 只写代码、测试、`docs/implementation/**` 偏差登记、协作线程中的“进展/验证/问题”。
- **不改** `docs/specs/**`、`docs/adr/**`、`docs/po/**`、`docs/architect/**`。
- ADR 未定义/自相矛盾 → 在 `docs/collab/architect-developer/threads/` 开 `T-NNN`，按“现象 / Spec 章节 / ADR 章节 / 影响 / 选项与建议”提问；不自己发明规则。
- 偏差必须登记 `owner` + `expiry`；到期未清升级 BLOCKING。
- 禁止 `// TODO` 掩盖 E1…E25 / B-01 / B-05；禁止 `try-catch` 吞分布式事务异常。
- 每条 AC 至少一个 `TestAC<NN>` 测试（测试函数名含编号）；改名不删测试。

## 2. 当前基线（2026-11-22）

| 项 | 值 |
|---|---|
| Spec | SPEC-0001 **v1.2 approved** |
| ADR | ADR-0005 **v1.2**（§18 Amendment 1 / §19.2 T-001 closed）；ADR-0006 gates accepted |
| 实施线程 | `docs/collab/architect-developer/threads/T-001.md` **OPEN**，由 Architect 复核后关闭 |
| 实施提交 | `0c1bd16`（B-01/B-05/SPEC v1.2）、`655ccc7`（gates 自测标签）、`e81de00`（T-001 进展回写） |
| 偏差登记 | `docs/implementation/adr-0005-deviation-register.md` |
| 生产开关 | quantity split / carrier registry / auto confirm / shortage / freight responsibility 默认 false |

## 3. 已完成（不要重复实现）

- B-01：`AllocateOrderReply.availability`、`INVENTORY_WAREHOUSE_PRIORITY`、Order 下单复算、`availability_json`、`stock_changed` + BFF `next_preview`。
- B-02：候选排序 `(priorityRank,-available,warehouse_id)`。
- B-03：责任枚举/原因码/证据、168h 运费挂起、判责 RPC。
- B-04：授权号 + `VerifyRestockAuthorization` + Inventory fail-closed。
- B-05：缺货 4h/48h 字段、`shortage.resolved` 必填 payload、AfterSales 系统退款单 + durable refund task + Order 退款投影。
- SPEC v1.2：组号按 `warehouse_id` 升序、运费券只减订单级、60s 申诉宽限、15 天已确认争议不回滚、`AUTO_T7` 文案、新单禁止直接改额。
- ADR-0006：`make gates/gates-strict/gates-test`、AC baseline/coverage、CI job；A1…A5 自测通过。

## 4. 当前残余（有 owner + expiry）

| 项 | owner | expiry |
|---|---|---|
| `change_reason` / 退货 `return_carrier_code` | Developer | 2026-12-15 |
| `docs/contracts.md` 同步 §18.8 | System Architect | 2026-12-15 |
| 缺货选择/退款进度 C 端 BFF/UI | Developer | 2026-12-15 |
| 日终对账 / 真实 NATS/DTM/Payment 故障注入 | Developer | 2026-12-15 |
| C-01/C-02/C-03 兼容 shim | Developer | 2026-12-31 |

## 5. 易漂移点（改代码前先看）

1. 组号按 `warehouse_id` 升序，不按优先级/首次出现（P-15/AC-38）。
2. 运费券只减订单级；行优惠最大余数法；部分组只退 `freight_share`（P-01/02/05/18）。
3. 48h 从 `notified_at` 起算；4h 是系统计时上限；T+7=168h（P-04）。
4. 申诉优先（60s 宽限）；已确认后 15 天争议**不回滚 receipt**（P-07）。
5. 责任 `PLATFORM` 视同商家；自由文本 `reason` 不判责（INV-29）。
6. 缺货退款走 AfterSales durable task，不在 NATS handler 里打款。
7. 支付冻结：`status=PAID` 与 `allocation_frozen_at` 同事务；冻结后行/组/仓只读（INV-22）。
8. 新单拒绝直接改金额；历史 `line_no=0` 只走审计兼容路径（C-02）。

## 6. 常用命令

```bash
export PATH="$PWD/.tmp/bin:$PATH"
export GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp
export GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath
export GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off

make verify
make gates-strict
make gates-test
make contract-check
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true&loc=UTC' \
  go test ./internal/services/order -count=1
```

## 7. 新会话开场动作

1. 读 `AGENTS.md`、`docs/collab/CONVENTIONS.md`、Spec 当前版本、ADR-0005/0006、本文件。
2. 看 `docs/collab/architect-developer/threads.md` 与 T-001；有 OPEN 先处理。
3. `git status` 确认是否有其他 Agent 的在途改动；**不覆盖/不回滚**，只提交自己的路径。
4. 先跑 `make gates-strict` 与相关测试，再决定改什么。
5. 需要改 Spec/ADR 时开线程，不直接编辑。
