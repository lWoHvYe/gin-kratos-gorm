# Spec 索引（入口）

本文件只是**索引**，不承载业务内容。所有规格说明书存放在 [`docs/specs/`](docs/specs/)，规范见 [`docs/specs/README.md`](docs/specs/README.md)。

## 当前有效

| 编号 | 标题 | 状态 | 版本 | 文件 |
|---|---|---|---|---|
| SPEC-0001 | 多仓按数量拆行、选仓优先级与承运商签收 | approved | 1.2 | [0001-fulfillment-quantity-split.md](docs/specs/0001-fulfillment-quantity-split.md) |

## 新增 Spec 的命名规则

```text
docs/specs/NNNN-<domain>-<capability>.md
例：docs/specs/0002-ai-turn-action.md
```

- `NNNN` 四位递增、永不复用；`domain` 取自 README 第 1.1 节词表；`capability` 用英文 kebab-case，≤ 4 词。
- 文件名不写版本/日期/状态；版本与状态写在文首 YAML，并在 [`docs/specs/README.md`](docs/specs/README.md) 第 4 节登记。
- 一个文件只写一个业务能力；已批准后改口径是**原地升版**，不是新建文件。
- **approved 即入库**：批准后当轮 commit，格式 `docs(spec): SPEC-NNNN vX.Y <摘要>`，改动可追踪。

## 交接顺序

```text
Spec（approved） -> ADR -> 代码与测试 -> Reviewer 对照 Spec 审查
```

Developer 在 ADR 获批前不得开工。
