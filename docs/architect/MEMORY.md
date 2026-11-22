# Architect 记忆区

> 所有者：System Architect。其他角色只读。
> 用途：跨会话保留**设计结论的索引、未完成项、易漂移点**，避免重新翻 1000 行 ADR。
> 规则：这里不放业务规则；业务结论以 `docs/specs/**` 为准，设计结论以 `docs/adr/**` 为准。本文件只是指针 + 状态。

## 工作守则（每轮开场先做）

1. 先看 `docs/collab/*/threads.md`：有 **OPEN** 线程先答（按“现象 / Spec 章节 / ADR 章节 / 影响 / 选项”五要素）。若 OPEN 是自己发出的实施交接，等对方，不自问自答。
2. 已 **ANSWERED** 的线程：先回写 ADR（用 **Amendment**，不原地改写已接受结论），再按线程自身的关闭条件标 `CLOSED`、填 `closed_by`、同步索引。
3. 若线程关闭条件还要求实现/验证（例如 T-002 的 A1…A5），**不得提前关闭**；保持 `ANSWERED`，在索引注明缺口，证据齐再关。
4. ADR 变更只用 Amendment / supersede；**不编辑 `docs/specs/**`**（Spec 归 PO）。业务口径要改 → 开线程给 PO。
5. 关闭前删除所有“待裁决 / 待确认”标记，核对线程索引与 `closed_by`；每轮结束更新本文件“当前基线 / 未完成项”。

## 当前基线

| 项 | 值 |
|---|---|
| Spec | SPEC-0001 **v1.2**（approved，`docs/specs/0001-fulfillment-quantity-split.md`） |
| ADR | ADR-0005 **v1.2 / Amendment 2**（accepted，T-001 已回写） |
| 流程 ADR | ADR-0006 协作闸门可执行化（accepted；A1…A5 PASS，Reviewer PASS `f162ecf`） |
| 线程（PO↔Arch） | T-001 **CLOSED**；T-002 **CLOSED**（A1…A5 PASS） |
| 线程（Arch↔Dev） | T-001 **CLOSED**（实施交接）；T-002 **CLOSED**（scope 追认） |
| Reviewer 门禁 | `docs/collab/developer-reviewer/T-001` **CLOSED / PASS**（`f162ecf`） |
| 生产开关 | `QUANTITY_SPLIT_ENABLED` / `CARRIER_REGISTRY_ENABLED` / `DELIVERY_AUTO_CONFIRM_ENABLED` / `SHORTAGE_CASE_ENABLED` / `FREIGHT_RESPONSIBILITY_ENABLED` 默认 false |

## 未完成项（不能判 done）

- **B-01/B-05**：Developer 已交付，Reviewer **PASS**（P0-1…P0-7 关闭）；发布门禁已过。
- **残余**：B-06a（change_reason/return_carrier_code）、C-01…C-03 shim、D-01/D-02（缺货 BFF、日终对账/故障注入），owner+expiry 见偏差登记；**到期未清升级 BLOCKING**。
- AC-22/28/29/30/35/37 的浏览器/SLA 深度缺口仍在 `ac-coverage.tsv`，`expiry=2026-12-15`；不得静默续期。
- 生产开关仍默认 false；灰度需单独决策。

## 易漂移点（每次改 ADR 先看）

1. **组号**：按 `warehouse_id` 升序，与业务优先级无关（P-15/AC-38）。
2. **金额**：行优惠最大余数法；运费券只减运费；运费优惠只作用订单级；部分组只退 `freight_share`（P-01/02/05/18）。
3. **时间**：4h/24h/48h/168h 全按小时；48h 从通知起算；7 天=168h（P-04）。
4. **签收**：申诉优先（同刻亦然，60s 宽限）；已确认后 15 天内可争议但**不回滚 receipt**（P-07）。
5. **补发**：本期不做，只退款（P-13）；不要留“补发开关”。
6. **超退**：INV-24 无例外，平台补偿另立项目（P-14）。
7. **责任**：`PLATFORM` 视同商家；reason_code 枚举封闭，不允许 `OTHER`；自由文本不判责（P-08/P-10/INV-29）。
8. **回库**：`restock_state=NONE` 表示不适用，禁止 NULL/空串（P-09）。
9. **库存变化**：BFF 同响应返回完整 `next_preview`，不得只回 `retry_preview`（P-17）。
10. **冻结**：支付确认事务内 `allocation_frozen_at`；冻结后行/组/仓只读（INV-22）。

## 闸门（ADR-0006）

- `make gates`（warn，并入 `verify`）/ `make gates-strict`（CI/合并）/ `make gates-test`（A1…A5 自测）。
- 豁免只允许 `Role:`、`Exception: T-NNN`、`Non-normative: true`；不得删 AC/测试、改 baseline、堆豁免、改脚本恒真。
- 覆盖台账 `docs/collab/ac-coverage.tsv` 由 Reviewer 维护；baseline `docs/collab/ac-baseline.txt` 只增不减。

## 协作通道

- PO↔Arch：`docs/collab/po-architect/`
- Arch↔Dev：`docs/collab/architect-developer/`（实施交接/契约提问；trailer 用 `Thread: architect-developer/T-NNN`）

## 最近裁决（2026-11-22）

- Arch↔Dev T-002：scope 词表**追认（选项 B）**——`gates` 不含 `ac-baseline.txt`/`ac-coverage.tsv`（Reviewer 所有）；闸门自测用 `ci(gates)`，不用 `test(collab)`。
- `be11508` 由 Developer 直改 CONVENTIONS 属越权；内容追认，历史不回改；后续改 CONVENTIONS 必须开 `architect-developer/T-NNN`。
- B-07a：`docs/contracts.md` 已补 `shortage.reported` 时间字段与 `aftersale.completed.responsibility_party`。

## 常用命令

```bash
make gates-strict
make verify
make contract-check
git log --grep 'Thread: T-00' --oneline
```

## 更新规则

- 每次 ADR Amendment / 线程状态变化 / 新 Spec 版本：更新本文件“当前基线”和“未完成项”。
- 不复制 ADR 正文；只写指针和一句话结论。
- 业务口径不确定时，开 T-NNN 线程，不在这里定规则。
