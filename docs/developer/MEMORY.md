# Developer 工作记忆（长期）

> 所有者：Developer。其他角色只读。
> 只收跨变更仍成立的稳定经验；运行状态见 `docs/review-tracking.md` / `ac-coverage.tsv`；协作闸门已于 2026-11-25 退役，pair 线程已退役（2026-11-25）。
> 只放“下次开新会话必须记得”的角色边界、易漂移点与命令；**不承载业务规则**。业务以 `docs/specs/**` 为准，设计以 `docs/adr/**` 为准。

## 1. 角色与硬边界

- 只写代码、测试、`docs/implementation/**` 偏差登记、协作交付说明中的“进展/验证/问题”。
- **不改** `docs/specs/**`、`docs/adr/**`、`docs/po/**`、`docs/architect/**`。
- ADR 未定义/自相矛盾 → 停止受影响交付并向 Lead 提出，附“现象 / Spec 章节 / ADR 章节 / 影响 / 选项与建议”；不自己发明规则。
- 偏差必须登记 `owner` + `expiry`；到期未清升级 BLOCKING。
- 禁止 `// TODO` 掩盖 E1…E25 / B-01 / B-05；禁止 `try-catch` 吞分布式事务异常。
- 每条 AC 至少一个 `TestAC<NN>` 测试（测试函数名含编号）；改名不删测试。

## 2. 易漂移点（改代码前先看）

1. 组号按 `warehouse_id` 升序，不按优先级/首次出现（P-15/AC-38）。
2. 运费券只减订单级；行优惠最大余数法；部分组只退 `freight_share`（P-01/02/05/18）。
3. 48h 从 `notified_at` 起算；4h 是系统计时上限；T+7=168h（P-04）。
4. 申诉优先（60s 宽限）；已确认后 15 天争议**不回滚 receipt**（P-07）。
5. 责任 `PLATFORM` 视同商家；自由文本 `reason` 不判责（INV-29）。
6. 缺货退款走 AfterSales durable task，不在 NATS handler 里打款。
7. 支付冻结：`status=PAID` 与 `allocation_frozen_at` 同事务；冻结后行/组/仓只读（INV-22）。
8. 新单拒绝直接改金额；历史 `line_no=0` 只走审计兼容路径（C-02）。

## 3. 提交 scope

词表在 `docs/collab/CONVENTIONS.md` §5.1/§5.2。Developer 常用：

- 单服务：`feat|fix|test(order|pricing|inventory|fulfillment|aftersales|payment|gateway|admin|catalog|cart|identity|seckill|ai)`
- 契约：`feat(contracts)`；迁移：`feat(migrations)`
- 闸门：无（协作门禁已退役）；工具：`chore(tooling)`
- 记忆：`docs(developer)`；偏差登记：`docs(impl)`

单服务与契约同改时拆两笔，不混提。

## 4. 常用命令

```bash
export PATH="$PWD/.tmp/bin:$PATH"
export GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp
export GOMODCACHE=$PWD/.tmp/modcache GOPATH=$PWD/.tmp/gopath
export GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off

make verify
# 协作闸门已退役（2026-11-25）：不再运行 make gates*
make contract-check
TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/mall_order_test?parseTime=true&loc=UTC' \
  go test ./internal/services/order -count=1
```

## 5. 决策位置索引与开场动作

| 决策类型 | 位置 |
|---|---|
| 协作准则/分级/词表 | `AGENTS.md`、`docs/collab/CONVENTIONS.md` |
| 业务规则 | `docs/specs/**`、`docs/po/**` |
| 设计/协议 | `docs/adr/**`、`docs/architect/**` |
| 运行状态/交付跟踪 | `docs/review-tracking.md` |
| AC 覆盖台账 | `docs/collab/ac-coverage.tsv` |
| 实现偏差 | `docs/implementation/**` |

开场：

1. 读 `AGENTS.md`、`docs/collab/CONVENTIONS.md`、Spec 当前版本、ADR 与偏差登记。
2. `git status` 确认是否有其他 Agent 的在途改动；**不覆盖/不回滚**，只提交自己的路径。
3. 先跑 `make test` / `make verify`（不含 gates），再决定改什么。
4. 需要改 Spec/ADR 时停止并上报 Lead，不直接编辑。
