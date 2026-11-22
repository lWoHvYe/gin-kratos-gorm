# Architect 记忆区

> 所有者：System Architect。其他角色只读。
> 用途：跨会话保留**设计结论的索引、未完成项、易漂移点**，避免重新翻 1000 行 ADR。
> 规则：这里不放业务规则；业务结论以 `docs/specs/**` 为准，设计结论以 `docs/adr/**` 为准。本文件只是指针 + 状态。

## 当前基线

| 项 | 值 |
|---|---|
| Spec | SPEC-0001 **v1.2**（approved，`docs/specs/0001-fulfillment-quantity-split.md`） |
| ADR | ADR-0005 **v1.2 / Amendment 2**（accepted，T-001 已回写） |
| 流程 ADR | ADR-0006 协作闸门可执行化（accepted，待 Developer 实现） |
| 线程 | T-001 **CLOSED**；T-002 **ANSWERED**（等实现 + A1…A5） |
| 生产开关 | `QUANTITY_SPLIT_ENABLED` / `CARRIER_REGISTRY_ENABLED` / `DELIVERY_AUTO_CONFIRM_ENABLED` / `SHORTAGE_CASE_ENABLED` / `FREIGHT_RESPONSIBILITY_ENABLED` 默认 false |

## 未完成项（不能判 done）

- **B-01**：`order_allocation_decisions.availability_json` 仍为 `{}`；Order 下单复算 + `single_warehouse_max` + 审计未完成。
- **B-05**：缺货只到 `RESOLVED_REFUND`，AfterSales 消费者/系统退款/可见性未完成；AC-30 不得通过。
- **T-002**：闸门脚本、baseline、coverage 台账、自测、CI 未实现；Reviewer A1…A5 未跑。
- AC-31…38 的测试需按 v1.2 补齐；AC-14 六场景。

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
