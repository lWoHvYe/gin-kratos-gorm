# Spec Kit / DSH 集成升级与模板维护

> 按需读，非规范。适用于升级 `specify` CLI、DSH 集成、bundle，或重新生成模板时。
> 当前基线：`specify 1.4.5`；DSH integration `1.4.5`；preset `gin-kratos-spec 0.2.0`；bugfix bundle `1.0.3`。

## 1. 先理解三层，不要混

| 层 | 位置 | 谁维护 | 升级风险 |
|---|---|---|---|
| CLI + 官方模板/脚本 | `~/.local/bin/specify`、`.specify/scripts`、`.specify/templates` | Spec Kit | 会覆盖/漂移 |
| DSH 集成 | `.dsh/skills/speckit-*`、`.specify/integrations/dsh.manifest.json`、`.specify/integration.json` | Spec Kit integration | 会重生成 skills |
| 项目定制 | `.specify/presets/gin-kratos-spec/**`、本仓 core 文件 | 本项目 | 必须保留/重放 |

当前 `specify integration status` 会报 **5 个 managed template 已修改**；这是本项目的刻意定制，不是故障。

## 2. 升级前（必须）

1. 工作区干净：`git status --porcelain` 应为空。
2. 建分支/备份：`git switch -c chore/speckit-upgrade-YYYYMMDD`。
3. 记录当前状态：

```sh
specify --version
specify integration status
specify preset list
specify bundle list
sha256sum .specify/templates/*.md .specify/presets/gin-kratos-spec/*
```

4. 导出本地差异，保存到仓库外或临时分支：

```sh
git diff 6ee05df..HEAD -- .specify/templates .specify/presets > /tmp/speckit-local.patch
```

5. 确认当前 5 个本地模板 delta（见 §5）；不要直接 `--force`。

## 3. 升级 specify CLI

```sh
specify self check
specify self upgrade --dry-run
specify self upgrade
specify --version
specify check
```

- `self upgrade` 只升级 CLI；不会自动处理本项目的模板/集成。
- 要固定版本用 `specify self upgrade --tag vX.Y.Z`。
- 不要用 `pip install -U` / `uv tool upgrade` 绕过 `specify self`，否则安装路径可能与当前 `specify` 不一致。

## 4. 升级 DSH 集成

```sh
specify integration status
specify integration upgrade dsh
```

- 默认是 **diff-aware**：发现本地模板修改会阻断；先看 findings。
- 只有确认已备份/已导出 patch 后，才考虑：

```sh
specify integration upgrade dsh --force
```

- `--force` 会覆盖 modified/unowned 文件，可能冲掉 §5 的本地模板 delta；不要把它当“重试”按钮。
- 升级后检查 `.dsh/skills/speckit-*` 是否重新生成；这些是生成物，不手改。
- `.specify/integrations/dsh.manifest.json` / `speckit.manifest.json` 是工具维护的 hash 清单，不手改；以 `integration status` 为准。

## 5. 本地模板 delta（升级后必须重放）

| 文件 | 本项目的修改 | 重放方式 |
|---|---|---|
| `.specify/templates/spec-template.md` | 增加 YAML `id/title/status/version/owner/supersedes/superseded_by/adr`；status 含 historical | 手工合并；保留 YAML |
| `.specify/templates/plan-template.md` | 目录树增加 `spec.md` | 手工合并 |
| `.specify/templates/tasks-template.md` | 行为变更测试/证据任务不再标 OPTIONAL | 手工合并 |
| `.specify/templates/checklist-template.md` | checklist 改为 author self-check，不作 reviewer gate | 手工合并 |
| `.specify/presets/gin-kratos-spec/spec-template.append.md` | 状态加 historical；豁免引用 v3 第 17 条 | 优先走 preset update |
| `.specify/presets/gin-kratos-spec/plan-template.append.md` | 增加 **ADR Constraints** 表 + 冲突停止 | 优先走 preset update |
| `.specify/presets/gin-kratos-spec/tasks-template.append.md` | 增加 **ADR Traceability**；声明 research/data-model/contracts 非规范 | 优先走 preset update |
| `.specify/presets/gin-kratos-spec/preset.yml` | 版本 0.2.0；新增 plan/tasks append | 随 preset 更新 |

原则：**能放 preset 的放 preset，核心模板改动越少越好**；每次升级后重放并重新跑 `git diff`。

## 6. 升级 preset / bundle

```sh
specify preset list
specify preset update gin-kratos-spec --dev .specify/presets/gin-kratos-spec
# 或从发布包：
# specify preset update gin-kratos-spec --from <url>

specify bundle list
specify bundle update bugfix
specify bundle validate
```

- 如果 preset 更新会覆盖本地 append，先备份 `.specify/presets/gin-kratos-spec/`。
- bundle 只更新其贡献的 extension/workflow；不要让它改写 `AGENTS.md`/宪法/Specs。

## 7. 升级后验证

```sh
specify integration status          # 期望只剩已知 delta；不应有 missing/manifest error
specify preset resolve spec-template
specify preset resolve plan-template
specify preset resolve tasks-template
specify preset list
specify bundle validate
git diff --check
```

再检查：

- `specify integration status` 的 modified 列表是否只含 §5；
- `.dsh/skills/speckit-*` 是否齐全；
- `specs/README.md` 仍是唯一 active index；`docs/specs/` 仍只有重定向；
- 新 Spec 模板渲染仍含 YAML + 附录 A；
- 不出现 `docs/specs/0001|0002` 的 live normative 引用。

## 8. 回滚

- 升级前已有分支/commit；失败直接 `git switch` 回原分支。
- 已 `--force` 但未提交：`git restore .specify .dsh`；不要手工改 manifest。
- 已提交：revert 升级 commit，保留本地模板 patch。

## 9. 禁止事项

- 不手改 `.specify/integrations/*.manifest.json`。
- 不在 dirty worktree 上 `--force`。
- 不把 `.dsh/skills/**` 当项目源文件修改；如必须改，按本地 delta 记录并在升级后重放。
- 不把 `docs/guides/**`、`docs/checklists/**` 当验收依据。
