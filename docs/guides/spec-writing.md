# Spec 写作指南（非规范）

> 操作辅助，冲突以宪法/AGENTS 为准。语法不限 EARS 或 Spec Kit 原生；目标是可判定、可验收。

## 1. 一条 AC 的最低要求

- 触发态：WHEN/IF/WHILE 或 Given；
- 主体：系统/服务；
- 行为：SHALL/MUST/THEN；
- 结果：可观测结果/错误码/状态；
- 拒绝：金额、状态、并发必须写拒绝错误；
- 证据：映射到测试/契约检查/可观测指标。

反例：用户退款时，如果金额超过支付，系统应报错。
正例：

```text
IF 申请金额 > captured 金额，
THEN THE SYSTEM SHALL 拒绝申请并返回 OVER_REFUND_ERROR；
AND 不改变 refund/commitment/permit 状态。
```

## 2. AC 编号与验收表

- 新 Spec：`<change_id>-AC-<n>`；长期 Spec：`AC-<n>`；退役编号不复用。
- 验收表列：AC、状态（未验证/有引用/已验证）、检查类型、证据路径、审查者。
- `AC-REF` 不等于 `AC-VERIFIED`；只有审查者能改状态。

## 3. 不要写进 Spec 的东西

- intent 表、permit 算法、幂等键、事件 schema、补偿协议 → ADR/plan。
- “快速/及时/合理”等不可判定形容词。
- 没有 AC 的“条款”当验收；必须有具体 AC。
- 避免用 if-else/flag 默认值描述兼容；写默认切换、收口、移除、回滚。

## 4. 异常分支清单

至少问：

- 本地提交后崩溃？
- RPC 成功后崩溃？
- 重复/乱序/延迟？
- 取消/确认竞态？
- 外部重放丢失？
- 超时/部分成功？
- 权限/库存/资金不足？
- 迁移/回滚？

每条写预期行为 + 恢复所有者；没有所有者的“可重试”不算设计。

## 5. Spec Kit 原生映射

- User Scenarios 的 Given/When/Then 可以当 AC；每条被实现/验收依赖的 scenario 必须映射到验收表。
- FR/SC 不能替代 scenario 的验收映射。
- 用 Spec Kit 模板时，末尾加“附录 A 对照”短表；不要复制宪法全文。
