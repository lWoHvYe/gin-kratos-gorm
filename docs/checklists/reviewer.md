# Reviewer checklist（非规范）

> 操作辅助，冲突以宪法/AGENTS 为准。审查者仍是同一角色：独立、可阻断、可溯源、不改代码/不给补丁。

## 0. 开始前

- [ ] 我是独立 actor，不是作者；没有参与被审变更的写作。
- [ ] 已读宪法、AGENTS、当前 Spec/ADR；只依据 base/head 之间的代码、plan、测试日志，不依据作者自述。
- [ ] 已记录精确 `base..head`、`review_scope`、`release_status`、`verification`。
- [ ] 作者没有提供自检/测试范围/未覆盖/取舍 → 证据不足，退回。

## 1. 范围与阻断

- [ ] 每条阻断问题能指向 AC、INV、已批准 ADR、Spec 必备章节或宪法条款；摘不到的记 `[TODO]`，不阻断。
- [ ] 权限、安全、数据完整性、迁移风险必须阻断，不因“没有测试”降级。
- [ ] 设计/架构问题：回 Spec/ADR；同一问题第二次不过且判为设计问题，按宪法第 5/31 条冻结。
- [ ] 不用轮次上限替代阻断；不因“审了几轮”放过资金/库存/权限/迁移问题。

## 2. Spec/ADR 审查

- [ ] 附录 A 全部八项都有落点，不是只看业务/AC。
- [ ] 状态所有权、契约/异常恢复、可达性、假设、业务裁决题、验收表、延期表。
- [ ] 命中附录 B 的事项有 ADR，且 ADR 与 Spec 同等审查/批准；plan 不能替代 ADR。
- [ ] 每条 AC 可判定，能落到测试/契约检查/可观测指标；无 AC 的“条款”不算验收。
- [ ] 5 个崩溃窗口有预期行为 + 恢复所有者（见 `design.md`）。
- [ ] 没有 if-else/局部锁/flag 默认 false 绕过协议；没有 TODO/静默 catch。

## 3. 代码审查

- [ ] 代码符合已批准 Spec/ADR；不是“测试过了就算”。
- [ ] 精确 base/head；HEAD 变了要重新确认差异和受影响验证。
- [ ] 测试是否独立 oracle？是否只是复述实现？高风险是否有 negative control/mutation/race/fault。
- [ ] 是否使其他 Spec 的已验证 AC 失效？列出 AC + 非回归证据。
- [ ] diff 大小是否可审；超过 200–400 行考虑拆分，不微观挑 Lint。
- [ ] 不允许“加个锁/加个 if”式修补；设计缺陷标 `[ARCHITECTURAL_FLAW]` 打回 Spec/ADR。

## 4. 裁决与状态

- [ ] PASS 只表示声明范围内证据满足，不是 Spec 全量验收/Release。
- [ ] AC 状态只由我写；作者不得自标 `[x]`。
- [ ] 台账/验收表写精确路径、HEAD、AC 位移；不写“已修”。
- [ ] 检查 commit trailer：未审必须 `Reviewed-by: pending`；伪造 trailer 直接 FAIL。

## 5. 不要做

- [ ] 不替作者写 Spec/ADR/代码，不给补丁建议。
- [ ] 不把主观设计担忧降为不阻断；能指向 INV/ADR/AC 的就阻断。
- [ ] 不因“作者说没问题”放行；不看作者自述当证据。
- [ ] 不把 `analyze`/`checklist`/`bug-test` 当验收。
- [ ] 不要求作者写 2,000 行 markdown；只读权威 Spec、变更段、plan/evidence、diff。
