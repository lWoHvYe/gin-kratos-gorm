# Architecture review tracking

Source: external review received 2026-11-18. Status is updated as items land.
`done` means code + verification; `partial` means a safe subset shipped and the
remaining risk is documented.

## P1

| # | Item | Status | Notes |
|---|---|---|---|
| 1 | Seckill TCC cancel bucket/mapping compensation | done | HELD/CONFIRMED paths implemented; closed-bucket HELD now writes normal ledger + stock.changed; MySQL tests cover open confirmed and closed late release. |
| 2 | Late release after activity RELEASED writes available | done | Both closed-bucket paths now write normal reserved, inventory ledger and stock.changed. |
| 3 | Qualification vs order release double-credit | partial | Guards and a sequential MySQL test exist; inventory stock events now carry version and catalog conditions `stock_version`; true concurrent race test still missing. |
| 4 | MarkPaid/cancel race leaves CONFIRMED stock | done | Normal/seckill cancel compensation runs; partial multi-SKU confirm is rolled back; race injection still desirable. |
| 5 | Coupon lock lost on crash | done | `locked_at`, TTL sweeper, pricing/0008 migration; live sweep verified. |

## P2

| # | Item | Status |
|---|---|---|
| 6 | Outbox dead-letter archive | done | prune now selects one id batch, archives/deletes those ids in one transaction; MySQL integration test passes |
| 7 | Outbox serial relay / lease timeout | done | batch 20, 4m lease (worst case 200s), backlog/oldest gauges; MySQL lease test |
| 8 | BFF 15s vs saga 30s timeout | done | checkout/place gets 35s; other routes 15s |
| 9 | quotes/inventory_ledger lifecycle | partial (quotes done; ledger deferred) | quote child/parent deletion is one transaction; inventory_ledger rows/age measured; archive policy still pending |
| 10 | Catalog SQL fallback vs GORM divergence/N+1 | done | batch loading, SKU price predicate and public/admin keyword predicates aligned; SQL fallback integration test still absent but code paths covered by shared helper |
| 11 | Deep OFFSET pagination | done (page > 1000 rejected; keyset still future) |
| 12 | LIKE '%x%' cannot use index | done | ngram fulltext index; >=2-rune keywords use MATCH, short keywords keep LIKE |
| 13 | Per-SKU GetStock in checkout/cart | rejected | only cart add and admin product detail call GetStock; checkout uses quote lines, no per-SKU loop (rechecked 2026-11-19) |
| 14 | AI frontend entry when disabled | done | public /ai/status drives the storefront entry |
| 15 | Base chart mock payment defaults | done | base mock=false; single-node overlay mock=true; payment secret required when mock off |
| 16 | P1 compensation/race tests | partial | seckill open/closed/cross-entry, partial-confirm compensation and stock ledger/outbox tests landed; no true concurrent race/chaos harness |
| 17 | Demo seed inside production migration | rejected | catalog migrations contain no INSERT; demo data is deploy/seed/catalog-demo.sql, applied only by make seed-demo |
| 18 | CORS reflects any origin when unset | done (fail closed; compose dev allowlist, chart value) |
| 19 | Admin identity/ban surface | partial | ListUsers/SetUserStatus + UI, refresh-token revocation and guards; already-issued access tokens remain valid until their 15m TTL. |

## P3

- Alertmanager/zero-alert rules: done for compose (6 rules, Alertmanager UI, default receiver); k8s observability stack still not packaged.
- Single-proto regeneration noise: accepted tradeoff (one shared proto).
- Compose etcd default: done (removed from default services; optional etcd-config docs remain).
- BFF skeleton duplication / 1300-line admin: not adopted in this pass (refactor only, no behavior defect).
- Duplicate migration scripts: not adopted (compose/k8s runners remain separate by design).
- Docs drift: AI MaxStep and MySQL version fixed; reviewed other counts.
- Frontend lint/test placeholders: not adopted (Vitest tests exist; ESLint toolchain not installed in this pass).
- k8s observability addon: not adopted; compose observability has alerts/Alertmanager, k8s stack is separate.
- Missing version/updated_at on some tables: partial (columns added; category/brand/SKU/address updates do not yet bump version).
- Proto reserved/deprecated: not adopted (no removed field numbers today; needs a CI evolution guard).
- preview_campaign fixed user_id=1: done (uses request identity).
- Duplicate frontend types / pinia version mismatch: pinia aligned to 4.1.0; duplicate hand-written/generated types still backlog.

## AI suggestions

- Real-model eval harness: done (13 cases).
- Feedback capture + quality dashboard: done (`/ai/quality`, admin summary RPC).
- RAG hybrid/rerank: done (BM25 + KNN + RRF + lexical rerank); semantic cache deferred.
- Eino interrupt-resume HITL: done (ADK checkpoint + admin draft confirm).
- Full ADK replacement: done (customer + admin ChatModelAgent; ReAct removed; eval 13/13).
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

## Second-pass audit (2026-11-19)

The following previously-done items were downgraded after re-reading the code,
not just the trackers:

- `ReleaseSeckillHold` and `releaseSeckillMapping` closed-bucket branches update
  `inventory_stocks.reserved` but never call `appendStockChanged`; the catalog
  stock projection can diverge. The former also has no normal inventory ledger.
- `refundCancelledAfterConfirm` restocks normal orders only; it skips
  `quote_id` starting with `seckill:`, so the non-TCC MarkPaid cancellation race
  is still uncompensated for seckill orders.
- P1 #3's MySQL test is sequential, not a concurrent race.
- Catalog SQL fallback has no integration test; it is code-complete only.
- The ADK HITL pilot uses a deterministic `draftReviewAgent`, not a
  `ChatModelAgent`; ADK full replacement applies to the chat runtime.
- Hybrid RAG's rerank is a deterministic query-token rerank; semantic cache is
  still deferred.
