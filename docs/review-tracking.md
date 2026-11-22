# Architecture review tracking

This is the canonical running status ledger. Update it every round with evidence
and remaining gaps. `docs/handoff.md` is a session-boundary snapshot and is only
refreshed when a new conversation is started.

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
| A (#20/#21/#24/#36) | done (dev cluster) | Order lifecycle is separate from `refund_state`/projection; shipping permit CAS + refund admission; server-owned `available_actions`; after-sales lifetime ledger from Payment captured, immutable effective-amount versions, `RETURN_RECEIVED`, durable restock/refund tasks, no status scanner; Payment `GetPaymentByOrder`; Fulfillment `Ship` requires the permit. MySQL tests cover permit/refund race, projection/lifecycle separation, TCC cancel, concurrent amount admission, restock failure/retry, refund retry/idempotency. Live dev verification: real-data preflight clones then Helm migration job complete; 17 legacy restock tasks converged to `DONE`; refund-only after shipment kept `SHIPPED/FULL`; pre-ship refund blocked shipment (`422`, permit `REVOKED`); the admin CREATE shipment path now checks `Order.available_actions` before inserting a shipment; `make dtm-smoke` passed; E2E phase15 passed; Playwright 7/7. Remaining: production backup/restore and rollback rehearsal; order detail still reads after-sales from the separate endpoint. |
| #3 catalog rebuild | covered by B | Version CAS + stale/conflict reporting from the previous slice. |

## 2026-11-21 product decision: multi-warehouse C

| Item | Status | Evidence / remaining gap |
|---|---|---|
| C cross-warehouse split orders | done (dev+UAT); production split flag intentionally off (no large-scale rollout) | ADR [0002](adr/0002-cross-warehouse-split-orders.md) accepted. Phase 1 live: `order/0011` applied after real-data preflight; 94 orders -> 94 `G1`/warehouse-1 groups, 0 missing groups, 0 bad item allocations; a new smoke order got `G1/1/ALLOCATED` and line 1/warehouse 1; `TestFulfillmentGroupShadowTracksOrder` passes. This is shadow state only: no split allocation, quote/pricing warehouse input, per-group permit/shipment or inventory key change yet. Phase 2a adds read-only `Inventory.AllocateOrder` (lowest warehouse id with full line availability, no reservation writes) and a gateway `WAREHOUSE_ALLOCATION_SHADOW` dual-read (500ms cap, log-only). Live dev: shadow enabled, `dtm-smoke` preview logged "warehouse allocation shadow matched single warehouse" (1 line), and E2E phase15 passed. Phase 2b adds `QuoteRequest.allocations`, `QuoteLine.warehouse_id/group_no`, `quote_groups`, per-group freight, and an Order gate that rejects split quotes until phase 3. Phase 3a makes inventory reservation/operation identity warehouse-aware: unique `(order_no, sku_id, warehouse_id)`, operation items keyed by sku+warehouse with `inv:v2` ids, OrderItem carries warehouse/group, and saga/Confirm pick it up. Live: `inventory/0016` preflight + migration passed; E2E phase15 passed. Phase 3b implements multi-group `CreateOrderInner` groups, Order/Gateway `WAREHOUSE_SPLIT_ENABLED` (default false), per-group shadow allocation and a split-quote validator. Phase 4 makes shipments/permits group-keyed, creates one parcel per order group, and resolves partial shipment/receipt into `shipment_status`. Live dev validation used a temporary `WAREHOUSE_SPLIT_ENABLED=true`: two-warehouse quote (G1/1 + G2/2), two Auto shipments, G1 ship -> `ALLOCATED/PARTIALLY_SHIPPED`, G2 ship -> `SHIPPED/SHIPPED`, G1 receive -> `SHIPPED/PARTIALLY_RECEIVED`, G2 receive -> `COMPLETED/RECEIVED`; phase15 E2E passed. Phase 5 makes after-sale items and restock tasks group/warehouse-aware, requires one group per after-sale, validates a return against the group's SHIPPED/RECEIVED state (not the overall partial order status), blocks only that group's permit, and restocks to the origin warehouse. Live dev: split order with G2 shipped; RETURN_REFUND for G2 restocked to warehouse 2 and refunded while G1 still shipped later to SHIPPED; permit G1 remained absent/unblocked. Phase 6 exposes group actions (`ship_group:`, `receive_group:`, `apply_after_sale_group:`), a storefront group/logistics/receive/after-sale UI, a group selector on the after-sale apply page, admin per-group shipment creation, and a shipment-list BFF. Live browser check: split order showed G1/G2, G2 receive moved the projection to PARTIALLY_RECEIVED, and the apply page carried the G2 group. Dev single-node now enables split; production defaults remain false. System review: [c-warehouse-review.md](c-warehouse-review.md). Live dev: `pricing/0010` applied after real-data preflight (88 quotes -> 88 groups, 0 missing/bad lines); E2E phase15 passed; the new quote is `G1/warehouse 1` with per-group freight. Pricing unit tests cover group freight/discount and the single-group gate. Defaults approved: flat per-warehouse freight, stock/priority allocator, line-level split only, origin returns, paid-unshipped via refund. |

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
