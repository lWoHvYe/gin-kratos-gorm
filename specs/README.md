# specs/ — active Spec index

一份业务能力只有一份 active Spec。状态/版本以 Spec YAML 为准；运行状态在 `docs/review-tracking.md`。

| ID | Title | Status | Version | Authority path | Owner | Supersedes | ADR |
|---|---|---|---|---|---|---|---|
| SPEC-0001 | 多仓按数量拆行、选仓优先级与承运商签收 | approved (historical reference; QC delta pending) | 1.5 | [specs/0001-fulfillment-quantity-split/spec.md](0001-fulfillment-quantity-split/spec.md) | Product Owner | 无 | ADR-0005 |
| SPEC-0002 | 支付结算与履约准入 | approved | 1.11 | [specs/0002-payment-settlement-admission/spec.md](0002-payment-settlement-admission/spec.md) | Product Owner | 无 | ADR-0007 / Amd1 |

- `SPEC-0001` 为 historical reference（QC delta 待复审）：行为/AC 定义保留；不再新增/改 AC；§17 表为历史状态，不作 current release evidence。
- `SPEC-0002` 为 active；AC 状态以本 Spec §15 为准。
- `specs/sdd-struct-align-01` 为 historical，不进入本 index，不再作为当前证据。
- 新工作 `specs/<feature>/spec.md`；变更原地更新；evidence 放 `specs/<id>/evidence/<change_id>/`。
- legacy `docs/specs/` 仅保留重定向；无 active Spec。
- 0001/0002 目前没有 `plan.md`：`--paths-only` 只读取路径；`--require-spec` 只用于有 plan 的 Spec；不伪造 legacy plan。
