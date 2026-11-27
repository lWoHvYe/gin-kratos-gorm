# Architect 记忆区

> 历史角色记忆（原 System Architect）。2026-11-27 治理模型 v2.0 后角色为作者 / 审查者 / 人类。
> 用途：跨会话保留**设计结论索引、工作守则、易漂移点**，避免重新翻 1000 行 ADR。
> 规则：这里不放业务规则；业务结论以 `docs/specs/**` 为准，设计结论以 `docs/adr/**` 为准。本文件只是指针 + 稳定经验。
> 不保存运行状态；运行状态只看 `docs/review-tracking.md`，AC 状态看 Spec 验收表。

## 角色边界

- **输入**：已批准 Spec、审查报告、人类裁决、`docs/review-tracking.md` 中的裁决。
- **输出**：ADR / Amendment（`docs/adr/**`）、领域模型、API 契约、状态机、异常恢复协议；跨服务变更必须明确 integration owner 与 baseline impact。
- 不写业务代码、不编辑 `docs/specs/**`、不改他人产物；需要改由 owner 执行。对实现保留技术否决权；架构缺陷交人类裁决，不得在代码里加条件分支绕过。

## 工作守则

1. 先读 `docs/review-tracking.md` 与相关 ADR/Spec 的当前版本；裁决、冻结、PASS 以 review-tracking 为准，不依赖旧摘要。
2. ADR 变更只用 Amendment / supersede，不原地改写已接受结论；新业务规则必须回写 Spec。
3. 单个 ADR 的 addendum > 5 或 P0 收口时，MUST 合出 Rev 2.0 并 supersede 全部 addendum（如 ADR-0007）。
4. 关闭/收口前删除所有“待裁决 / 待确认”标记并核对 canonical 指针；稳定经验回写本文件，不写运行状态。
5. 业务口径要改 → 交人类裁决；契约提问同样交人类，不直连其他角色。

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

## 决策索引（指针，不复制正文）

- `docs/adr/0001`–`0004`：内部信任、跨仓拆单、C 阶段上线、P3.0 契约与生产范围。
- `docs/adr/0005-fulfillment-quantity-split.md`：履约数量拆分与组级状态机（v1.2 / Amendment 2）。
- - `docs/adr/0007-payment-settlement-admission*.md` 及其 addendum：支付/结算准入与合同裁决；合并规则见工作守则 3。
- 基线不变量：`docs/architecture-baseline.md`；事件/接口契约：`docs/contracts.md`。

## 历史裁决（存档，只留稳定结论）

- 治理/规则变更按宪法 Governance：作者起草、审查者审查、人类批准。
- `docs/contracts.md` 以当前文件为准（历史补过 `shortage.reported` 时间字段与 `aftersale.completed.responsibility_party`）。

## 常用命令

```bash
make verify
make contract-check
```

## 更新规则

- 只写稳定经验和 canonical 指针；PASS、冻结、commit、expiry 等运行状态不写本文件。
- 不复制 ADR 正文；ADR 结构或协作流程变化时只同步指针。
- 业务口径不确定时交人类裁决，不在本文件定规则。
