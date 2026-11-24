# GV-S21 — P0a T-A3 BASE projection (98d4c41)

> BASE-only evidence for the P0a atomic commit. `post_change_result` is filled after the P0a commit.
> Verifier is the Director (per the recorded human decision).

| field | value |
|---|---|
| verifier | Director (Independent Verifier) |
| designated_by | Director |
| independence_evidence | Detached worktree `git worktree add --detach /tmp/govcost-base.<rand> 98d4c41300ae944f9bf791dc7b380bc1fdfd0e5e`; worktree `git status --porcelain -uall` clean; worktree HEAD `98d4c41300ae944f9bf791dc7b380bc1fdfd0e5e`; BASE tree `804902cceb908ddc8577173f5b4077c0e8961506`; captured output sha1 `34762ad2a213f2060542b47e275497e8e80e8e3c`; worktree removed with `git worktree remove --force` + `git worktree prune`. |
| timestamp | 2026-11-24T23:47:12+08:00 |
| base | 98d4c41 |
| head | 98d4c41 |
| baseline_result | PASS — `python3 scripts/collab_gates.py --mode=strict --range=98d4c41^..98d4c41` → exit 0 |
| post_change_result | pending P0a commit |
| baseline_digest | sha1:34762ad2a213f2060542b47e275497e8e80e8e3c (captured stdout+stderr) |
| base_tree_digest | 804902cceb908ddc8577173f5b4077c0e8961506 |
| ac_scope_evaluated | AC-01…AC-68 |
| ac_scope_excluded | AC-69…AC-112 |
| scope_filter_command | `git worktree add --detach <tmp> 98d4c41 && cd <tmp> && python3 scripts/collab_gates.py --mode=strict --range=98d4c41^..98d4c41` |
| g4_status | NOT-RUN(not-implemented-at-BASE) |
| GATE_NOT_RUN? | false |

## Captured output

```text
AC coverage: 43/68
G-3 WARN uncovered: AC-39, AC-41, AC-42, AC-43, AC-44, AC-45, AC-46, AC-47, AC-48, AC-49, AC-50, AC-52, AC-53, AC-54, AC-55, AC-56, AC-57, AC-58, AC-59, AC-60, AC-61, AC-62, AC-63, AC-64, AC-66
G-3 WARN AC-39 deferred until 2026-12-31 (37d)
G-3 WARN AC-41 deferred until 2026-12-31 (37d)
G-3 WARN AC-42 deferred until 2026-12-31 (37d)
G-3 WARN AC-43 deferred until 2026-12-31 (37d)
G-3 WARN AC-44 deferred until 2026-12-31 (37d)
G-3 WARN AC-45 deferred until 2026-12-31 (37d)
G-3 WARN AC-46 deferred until 2026-12-31 (37d)
G-3 WARN AC-47 deferred until 2026-12-31 (37d)
G-3 WARN AC-48 deferred until 2026-12-31 (37d)
G-3 WARN AC-49 deferred until 2026-12-31 (37d)
G-3 WARN AC-50 deferred until 2026-12-31 (37d)
G-3 WARN AC-52 deferred until 2026-12-31 (37d)
G-3 WARN AC-53 deferred until 2026-12-31 (37d)
G-3 WARN AC-54 deferred until 2026-12-31 (37d)
G-3 WARN AC-55 deferred until 2026-12-31 (37d)
G-3 WARN AC-56 deferred until 2026-12-31 (37d)
G-3 WARN AC-57 deferred until 2026-12-31 (37d)
G-3 WARN AC-58 deferred until 2026-12-31 (37d)
G-3 WARN AC-59 deferred until 2026-12-31 (37d)
G-3 WARN AC-60 deferred until 2026-12-31 (37d)
G-3 WARN AC-61 deferred until 2026-12-31 (37d)
G-3 WARN AC-62 deferred until 2026-12-31 (37d)
G-3 WARN AC-63 deferred until 2026-12-31 (37d)
G-3 WARN AC-64 deferred until 2026-12-31 (37d)
G-3 WARN AC-66 deferred until 2026-12-31 (37d)
SUMMARY G-1=0 G-2=0 G-3=0 mode=strict exit=0
```
