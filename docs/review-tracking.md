# Architecture review tracking

Source: external review received 2026-11-18. Status is updated as items land.
`done` means code + verification; `partial` means a safe subset shipped and the
remaining risk is documented.

## P1

| # | Item | Status | Notes |
|---|---|---|---|
| 1 | Seckill TCC cancel bucket/mapping compensation | done | HELD releases the bucket; CONFIRMED reverses into the open bucket or normal on_hand when closed; MySQL tests cover open bucket and closed-bucket late release. |
| 2 | Late release after activity RELEASED writes available | done | `ReleaseSeckillHold` and `releaseSeckillMapping` now return held units to normal `reserved` when the bucket is closed. |
| 3 | Qualification vs order release double-credit | done | `ReleaseSeckillHold` skips converted mappings; expiry skips `order_no` rows; MySQL test proves only the order release credits the bucket. |
| 4 | MarkPaid/cancel race leaves CONFIRMED stock | done | cancel-after-confirm calls inventory `TccCancelConfirm` (non-seckill). Race injection test still desirable. |
| 5 | Coupon lock lost on crash | done | `locked_at`, TTL sweeper, pricing/0008 migration; live sweep verified. |

## P2

| # | Item | Status |
|---|---|---|
| 6 | Outbox dead-letter archive | done | terminal dead letters copied to outbox_dead_letters before prune; MySQL integration test passes |
| 7 | Outbox serial relay / lease timeout | done | batch 20, 4m lease (worst case 200s), backlog/oldest gauges; MySQL lease test |
| 8 | BFF 15s vs saga 30s timeout | done | checkout/place gets 35s; other routes 15s |
| 9 | quotes/inventory_ledger lifecycle | partial (quotes done; ledger deferred) | quotes cleaned after 30d; inventory_ledger rows/age measured; archive/retention needs a data-policy decision |
| 10 | Catalog SQL fallback vs GORM divergence/N+1 | done | fallback batches products/SKUs/images/attrs; public and admin keyword predicates now match the GORM reader |
| 11 | Deep OFFSET pagination | done (page > 1000 rejected; keyset still future) |
| 12 | LIKE '%x%' cannot use index | done | ngram fulltext index; >=2-rune keywords use MATCH, short keywords keep LIKE |
| 13 | Per-SKU GetStock in checkout/cart | rejected | only cart add and admin product detail call GetStock; checkout uses quote lines, no per-SKU loop |
| 14 | AI frontend entry when disabled | done | public /ai/status drives the storefront entry |
| 15 | Base chart mock payment defaults | done | base mock=false; single-node overlay mock=true; payment secret required when mock off |
| 16 | P1 compensation/race tests | not adopted in this pass | seckill/open-closed/cross-entry DB tests landed; a chaos/fault-injection harness is a separate project |
| 17 | Demo seed inside production migration | rejected | catalog migrations contain no INSERT; demo data is deploy/seed/catalog-demo.sql, applied only by make seed-demo |
| 18 | CORS reflects any origin when unset | done (fail closed; compose dev allowlist, chart value) |
| 19 | Admin identity/ban surface | done | ListUsers/SetUserStatus RPCs, admin BFF + users page; disable revokes refresh tokens, last-admin and self-disable guarded |

## P3

- Alertmanager/zero-alert rules: done for compose (6 rules, Alertmanager UI, default receiver); k8s observability stack still not packaged.
- Single-proto regeneration noise: accepted tradeoff (one shared proto).
- Compose etcd default: done (removed from default services; optional etcd-config docs remain).
- BFF skeleton duplication / 1300-line admin: not adopted in this pass (refactor only, no behavior defect).
- Duplicate migration scripts: not adopted (compose/k8s runners remain separate by design).
- Docs drift: AI MaxStep and MySQL version fixed; reviewed other counts.
- Frontend lint/test placeholders: not adopted (Vitest tests exist; ESLint toolchain not installed in this pass).
- k8s observability addon: not adopted; compose observability has alerts/Alertmanager, k8s stack is separate.
- Missing version/updated_at on some tables: done (catalog categories/brands/skus, identity addresses, seckill qualifications).
- Proto reserved/deprecated: not adopted (no removed field numbers today; needs a CI evolution guard).
- preview_campaign fixed user_id=1: done (uses request identity).
- Duplicate frontend types / pinia version mismatch: pinia aligned to 4.1.0; duplicate hand-written/generated types still backlog.

## AI suggestions

- Real-model eval harness: done (13 cases).
- Feedback capture + quality dashboard: done (`/ai/quality`, admin summary RPC).
- RAG hybrid/rerank/semantic cache: not adopted in this pass (roadmap).
- Eino interrupt-resume HITL: not adopted (current draft/confirm flow is hand-rolled and tested).
- Cart task assistant / proactive admin brief / long-term memory / multi-agent: not adopted (product roadmap).

## Scope decisions

Accepted and fixed: P1 #1-#5, P2 #6-#12, #14, #15, #18, #19; #13 and #17
were rejected after code inspection. The selected P3 items (alerts, etcd
default, schema conventions, preview identity, pinia alignment, docs drift)
are done.

Deferred/not adopted with reasons: P2 #9 ledger archive (needs retention
policy), P2 #16 fault-injection harness, proto reserved/evolution guard, BFF
split, migration-runner dedupe, frontend ESLint tooling, duplicate hand-written
types, k8s observability addon, and the larger AI roadmap items (hybrid
retrieval/rerank, Eino HITL, cart assistant, memory, multi-agent).
