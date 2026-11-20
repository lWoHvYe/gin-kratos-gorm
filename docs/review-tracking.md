# Architecture review tracking

Source: 2026-11-20 handoff plus follow-up implementation. `done` means the code
and the stated verification exist; `partial` means a safe subset shipped and a
named gap remains. This file was rewritten after a 2026-11-21 audit because the
old table still described P2#10 as rejected and Phase 7 as not started.

## P1

| ID | Status | Evidence / remaining gap |
|---|---|---|
| P1#1 | done | Closed-bucket release writes one ledger/event; `TestReleaseTxClosedSeckillWritesOneLedger` (MySQL) proves exactly one row. |
| P1#2 | done | `order.cancelled` uses `TccCancelConfirm`; partial MarkPaid compensation includes the failing SKU; `afterPaid` failure emits a durable confirm event consumed by `RunConfirmConsumer`; NATS confirm/order.cancelled tests pass. |
| P1#3 | done | `Stock.version`; `cmd/catalog-stock-rebuild` dry-run found 3 mismatches and repaired 3. |
| P1#4 | done | Full draft SHA binding, fail-closed meta, close errors surfaced; live tamper 409, correct/duplicate 200. |
| P1#5 | done | `contains_any` executes; eval 15/15. |

## P2 (current handoff IDs)

| ID | Status | Evidence / remaining gap |
|---|---|---|
| P2#8 | done | Public GetProduct/GetSku require ENABLED + price>0. |
| P2#9 | done | page>1000 INVALID; cursor errors unified; GORM/SQL keyset walk to last page tested. |
| P2#10 | done | `BatchGetStock`; checkout/cart-merge/AI confirm batch prefetch + local totals; dedupe/missing test. The older “rejected” note is obsolete. |
| P2#11 | done | Assistant root is hidden until `available=true`. |
| P2#12 | done | Old broad cleanup disabled; `0014` requires exact seed fields/version; SQL positive/negative checks. |
| P2#13 | done | Strict identity for checkout/payment/admin; normal routes keep 5s cache; live warmed-cache disable→401. |
| P2#14 | done | `make test-integration` covers inventory/catalog/pricing and CI invokes it; local run passed. Real GitHub CI not run. |
| P2#17 | partial | Versioned index, dual-write, lease renewal, alias/cache invalidation, metrics, hit@5 5/5 done. No independent Job/elected leader; first build >3m can leave followers without alias; quality comparison cache/rerank still thin. |

## P3

- Version increments: done for catalog category/brand/SKU, identity address,
  seckill qualification.
- CampaignSpec: done.
- etcd: optional/disabled by default in chart, README corrected.
- Frontend ESLint: done; admin 0 error/11 warning, storefront 0 error.
- Admin router: split to 263 lines; promotion/catalog/orders/seckill/catalog-write
  handlers moved. Further business-boundary split remains optional.
- Generated types: read models Sku/Product/Order/AfterSale/Campaign/PromotionRule/
  Stock/Category/Brand derive from generated schemas; Shipment/Refund/ListReply
  still hand-written because no generated schema exists.
- proto guard: RPC signatures + reserved; baseline 680; CI invokes guard.
- K8s observability: OTLP collector egress added.

## AI roadmap truth table

| Phase | Code status | Honest status / gap |
|---|---|---|
| 5 correctness | identity, eval, HITL binding/atomic/idempotency, explicit CONFIRMING/SAVED | done for the gate; no FAILED/Abort state or UI state display |
| 6 versioned RAG | versioned index/alias, dual-write, lease renewal, reconciliation, cache, metrics, hit@5 | partial: no Job, first-build follower window, thin cache/rerank comparison |
| 7 cart assistant | draft tool, Redis draft, SSE draft, storefront confirm, Cart idempotent confirm | done for the implemented flow; no automated expired/price/stock-change rejection suite |
| 8 preferences | Redis profile, 180d retention, API/UI, system-message injection, A/B 3/3 | partial: no per-recommendation attribution record; benchmark is one prompt family |
| 9 ops copilot | deterministic daily brief, latest snapshot, atomic publication, low-stock links, feedback | partial: delivery is Redis snapshot/admin UI only; no richer anomaly explanations |
| 10 advanced | not adopted | deferred by evidence gate |

## Verification evidence

- `make verify` passes (proto guard included).
- `make test-integration` passed locally with MySQL/NATS/Redis across outbox,
  consumer, AI RAG, seckill, inventory, catalog and pricing.
- `ai-eval` 15/15; `ai-rag-eval` hit@5 5/5; `ai-pref-eval` 3/3.
- Phase 7 real-model draft SSE → confirm 200; normal answer emitted 246 SSE deltas.
- RAG lease renewal Redis test; HITL live meta state SAVED; strict revocation 401.

## Explicit unverified / drift

- Real GitHub Actions was not executed.
- RAG Job/elected-leader and crash/restart live drill are not done.
- HITL has no FAILED/Abort state or admin UI state display; CONFIRMING transition
  is not an atomic CAS.
- Phase 8 has no automated cross-user test or per-recommendation attribution.
- Phase 9 has no external delivery channel or rich anomaly explanations.
