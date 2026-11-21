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
| P1#4 | done | Full draft SHA binding, fail-closed meta, close errors surfaced, Lua CAS state transitions; live tamper 409, correct/duplicate 200, save-failure -> FAILED -> retry test. |
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
| P2#17 | partial | Versioned index, dual-write, lease renewal, alias/cache invalidation, metrics, hit@5 5/5 done. Independent `ai-rag-build` Job/CronJob + load-only follower Pods remove the first-build/follower window. Remaining gap: cache/rerank quality comparison still thin. |

## 2026-11-21 external review: transaction refactor B

| Item | Status | Evidence / remaining gap |
|---|---|---|
| B (#1/#2/#37) | done (dev cluster) | `inventory_operations`/`inventory_operation_items` (0015), deterministic operation id + scope hash, per-item progress, savepoint per item, terminal events, replay, DTM single-branch path. MySQL/NATS tests cover middle-item failure/resume, permanent per-item failure, mixed cancel, all-or-nothing branch tx, retry budget, concurrent confirm/cancel, replay. The inventory image now exposes `op-replay` as a one-shot recovery command. Live dev verification: Helm migration job complete; DTM smoke passed; multi-SKU live confirm/cancel reached 2/2 `SUCCEEDED`. Remaining: production rollout/rollback rehearsal; a gRPC/BFF replay surface is optional. |
| #22 payment config | done | Invalid `PAYMENT_MOCK_ENABLED`/AI booleans fail config loading; non-dev `PAYMENT_MOCK_ENABLED=true` is rejected; `PAYMENT_SECRET` remains required when mock is off. Config tests cover all three. |
| #38 AI mesh | partial | `values.yaml` has explicit `*-ai-read` rules for Catalog/Inventory/Pricing/Order/AfterSales; Helm render has no `ai-sa` wildcard. Live 403 verification was not run. |
| AI RAG release isolation | done (dev) | The post-upgrade RAG build was blocking upgrades when the AI image was stale; the one-shot Job is now opt-in (`ai.ragBuild.hook=false`), the CronJob is the normal path, and a manual `create job --from=cronjob` completed. Helm upgrade completed without `--no-hooks`. |
| Storefront HTTP session | done (dev) | The built cookie was `Secure` on plain HTTP, so a full page load dropped the session and the seckill browser test could not authenticate. Cookie security now follows the request protocol; Playwright 7/7 passed. |
| #3 catalog rebuild | done for version guard | `decideProjection` refuses a newer catalog projection and the SQL update is CAS-guarded; unit test covers equal/newer/stale/conflict. Missing-catalog/inventory rows are reported as counters, not a full reconciliation UI. |

## 2026-11-21 external review: transaction refactor A

| Item | Status | Evidence / remaining gap |
|---|---|---|
| A (#20/#21/#24/#36) | done (dev cluster) | Order lifecycle is separate from `refund_state`/projection; shipping permit CAS + refund admission; server-owned `available_actions`; after-sales lifetime ledger from Payment captured, immutable effective-amount versions, `RETURN_RECEIVED`, durable restock/refund tasks, no status scanner; Payment `GetPaymentByOrder`; Fulfillment `Ship` requires the permit. MySQL tests cover permit/refund race, projection/lifecycle separation, TCC cancel, concurrent amount admission, restock failure/retry, refund retry/idempotency. Live dev verification: real-data preflight clones then Helm migration job complete; 17 legacy restock tasks converged to `DONE`; refund-only after shipment kept `SHIPPED/FULL`; pre-ship refund blocked shipment (`422`, permit `REVOKED`); `make dtm-smoke` passed; E2E phase15 passed; Playwright 7/7. Remaining: production backup/restore and rollback rehearsal; order detail still reads after-sales from the separate endpoint. |
| #3 catalog rebuild | covered by B | Version CAS + stale/conflict reporting from the previous slice. |

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
- proto guard: RPC signatures + reserved; baseline 685; CI invokes guard.
- K8s observability: OTLP collector egress added.

## AI roadmap truth table

| Phase | Code status | Honest status / gap |
|---|---|---|
| 5 correctness | identity, eval, HITL binding/idempotency, explicit CONFIRMING/SAVED, FAILED via Abort, admin state UI, Lua CAS | done for the gate |
| 6 versioned RAG | versioned index/alias, dual-write, lease renewal, reconciliation, cache, metrics, hit@5, build Job/CronJob + load-only followers | partial: cache/rerank quality comparison thin |
| 7 cart assistant | draft tool, Redis draft, SSE draft, storefront confirm, Cart idempotent confirm | done: real-model E2E + expired/price/stock/unclosed Handler tests |
| 8 preferences | Redis profile, 180d retention, API/UI, system-message injection, A/B 3/3, per-session attribution key, Redis isolation test, attribution SSE event + storefront UI | partial: benchmark still one prompt family; add a cross-user/no-memory regression set |
| 9 ops copilot | deterministic daily brief, latest snapshot, signed webhook (HMAC, fail-closed, delivery id), low-stock links, feedback | partial: cancellation/refund anomaly explanations not built |
| 10 advanced | not adopted | deferred by evidence gate |

## Verification evidence

- `make verify` passes (proto guard included).
- `make test-integration` passed locally with MySQL/NATS/Redis across outbox,
  consumer, AI RAG, seckill, inventory, catalog and pricing.
- `ai-eval` 15/15; `ai-rag-eval` hit@5 5/5; `ai-pref-eval` 3/3.
- Phase 7 real-model draft SSE → confirm 200; normal answer emitted 246 SSE deltas.
- RAG lease renewal + follower pointer swap + build-due Redis tests.
- HITL Redis CAS tests: save-failure -> FAILED -> retry, SAVED terminal, illegal
  transition, missing/corrupt meta; admin BFF save-failure retry test.
- Attribution: owner-scoped SSE event (cross-user has none) and BFF proxy test.
- Webhook: signature/timestamp/delivery headers, fail-closed without secret,
  non-2xx rejection; frontend typecheck + build pass. `make verify` passes with
  proto guard at 685 baseline entries (the refresh also added previously missing
  HITL abort fields).
- Inventory B: `go test -race ./internal/services/inventory` with MySQL/NATS
  passed; `make test-integration` passed including the new operation tests;
  migration `0015` applied cleanly to a scratch MySQL 8.4 database.
- Transaction A: `go test -race ./internal/services/order
  ./internal/services/aftersales ./internal/services/payment
  ./internal/services/fulfillment` passed with MySQL; `make test-integration`
  includes the new order/payment/after-sales/fulfillment tests. Migrations
  `order/0010` and `aftersales/0010` were applied to scratch MySQL 8.4 data;
  the legacy status/amount/restock backfills produced the expected rows.

## Explicit unverified / drift

- Real GitHub Actions was not executed.
- The new AI image/Job/CronJob was not rolled out to the dev cluster in this
  session; the RAG crash/restart live drill is still pending.
- Phase 8 has owner-scoped attribution, but no automated cross-user/no-memory
  benchmark beyond the one prompt family.
- Phase 9 has a signed external webhook, but no rich cancellation/refund anomaly
  explanations.
- Inventory B has no live DTM barrier smoke (`make dtm-smoke`) or cluster
  rollout; `ReplayInventoryOperation` is an internal method without an admin
  RPC/BFF surface yet.
- Transaction A has no cluster rollout, live DTM smoke, or browser E2E; the
  new migrations were validated on scratch data, not against the deployed
  `mall_order`/`mall_aftersales` databases.
