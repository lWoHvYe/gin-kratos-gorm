# v3.0 adoption

Status: merged on main (human direction); independent review pending; release_status: NOT RELEASE.

## Scope

- 所有 active Spec 在 `specs/<id>/spec.md`；`specs/README.md` 是唯一 index。
- `0001/0002` 已从 `docs/specs/` 迁移；只改链接/路径，不改业务/AC。
- `0001` 为 frozen reference；`0002` 为 active；`sdd-struct-align-01` 为 historical。
- `docs/specs/` 只保留重定向 README；不保留副本。
- `Spec.md` 只做指针；ADR-0009 由 ADR-0012 取代。

## Rollback

- 把 `specs/0001.../spec.md`、`specs/0002.../spec.md` 移回 `docs/specs/`；
- 恢复 `Spec.md`/`specs/README`/`docs/specs/README`/ADR-0009 header/sdd status；
- 不留副本。
