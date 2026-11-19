# Architecture review tracking

Source: 2026-11-19 review (supersedes the earlier tracking table). `done` means
code + verification; `partial` means a safe subset shipped and the remaining
risk is explicit.

## P1

| ID | Status | Notes |
|---|---|---|
| P1#1 | done | Seckill allocate/release/hold/TCC-cancel paths that change normal reserved now write inventory ledger + `stock.changed`; closed-bucket HELD covered by MySQL test. |
| P1#2 | done | Multi-SKU partial Confirm is rolled back; cancel compensation includes seckill; failed direct restock emits a durable retry event; consumer handler re-restocks once (MySQL test, repeated delivery is idempotent). |
| P1#3 | done | Stock events carry inventory `version`; catalog projects conditionally via `skus.stock_version`; MySQL test proves an older event cannot overwrite a newer projection. |
| P1#4 | done | HITL checks draft binding before save; campaign+rule are one pricing transaction; checkpoint closes only after the business write. Live wrong-payload 409, correct 200, replay 404. |
| P1#5 | done | Eval executes `contains_any` and rejects assertion-less cases; real eval 13/13. |

## P2

| ID | Status | Notes |
|---|---|---|
| P2#6 | done | Outbox prune selects one ID batch and archives/deletes those IDs in one transaction; MySQL test passes. |
| P2#7 | done | Quote cleanup is one transaction; inventory_ledger has a 180-day archive table/worker with MySQL test; growth metrics retained. |
| P2#8 | done | SQL fallback SKU predicate now includes `price_cent > 0`, matching GORM. |
| P2#9 | done | BFF/catalog page cap; public product list supports opaque keyset cursor (price/id), verified no overlap; admin lists keep page cap. |
| P2#10 | rejected | Rechecked: only cart add and admin product detail call `GetStock`; checkout uses quote lines. No checkout/cart-merge per-SKU loop exists. |
| P2#11 | done | AI entry defaults hidden and remains hidden on status failure/non-2xx. |
| P2#12 | done | Forward cleanup migrations remove exact smoke seed; demo data remains an explicit `deploy/seed` script. Applied: smoke SPUs now 0. |
| P2#13 | done | `corsAllowedOrigins` is wired to gateway/admin Deployment env (verified in cluster). |
| P2#14 | done | `users.token_version` in JWT; status changes bump it; admin + high-risk checkout/payment BFFs revalidate. Live old demo token -> 401 after disable. |
| P2#15 | done | Concurrent seckill release, expired outbox lease, foreign relay owner, and durable compensation retry tests cover the targeted race/recovery paths; a full chaos suite remains future work. |
| P2#16 | done | Preview uses the same `userIDFromContext` as tools and errors when identity is missing; no fixed user 1. |
| P2#17 | partial | Versioned index/alias/full pagination + dual-write + post-build/15m reconciliation + exact/semantic cache + document/build/version metrics; true event high-watermark/ordering still open. |
| P2#18 | done | topologySpreadConstraints added; PDBs cover gateway/seckill/order/inventory/identity/catalog/pricing/payment; HA template renders. |
| P2#19 | done | Alertmanager added to observability target; Prometheus scrapes ai:9111; compose config valid. |

## P3

- Version increments: done for catalog category/brand/SKU, identity address and
  seckill qualification updates.
- CampaignSpec unification: pending (draft/preview/HITL still assemble rule JSON
  independently).
- Compose etcd: removed from `INFRA_SERVICES` and default docs; optional overlay
  remains for external etcd.
- Admin router size/client init duplication: not adopted.
- Ops DTM description: corrected (cancel is local + outbox; direct MarkPaid only
  when TCC disabled/unconfigured).
- Frontend lint placeholder, hand-written/generated type duplication, proto
  evolution guard, k8s observability addon: not adopted.
- Single proto regeneration and two migration runners: accepted tradeoffs.

## AI roadmap

| Phase | Status |
|---|---|
| 5 Correctness foundation | done: identity/eval/HITL binding+atomic save + CampaignSpec |
| 6 Versioned RAG | partial: versioned index/alias/full pagination + dual-write + post-build/15m reconciliation + exact/semantic cache + index metrics done; event high-watermark still open |
| 7 Cart assistant | not started |
| 8 Personalization | not started |
| 9 Proactive ops | not started |
| 10 Advanced (multi-agent/multimodal/routing) | not started |

## Verification run

- `make verify`
- `make ai-eval`: 13/13 with real assertions
- MySQL: outbox archive/lease, quote cleanup transaction, seckill
  open/closed/cross-entry/concurrent release
- HITL live: binding 409, confirm 200, replay 404
- Token revocation live: old access token 401 on checkout after disable
- `helm template` HA and `docker compose config` exit 0
