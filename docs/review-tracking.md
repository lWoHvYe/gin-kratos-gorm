# Architecture review tracking

This is the canonical running status ledger. Update it every round with evidence
and remaining gaps. `docs/handoff.md` is a session-boundary snapshot and is only
refreshed when a new conversation is started.

Source: 2026-11-20 handoff plus follow-up implementation. `done` means the code
and the stated verification exist; `partial` means a safe subset shipped and a
named gap remains. This file was rewritten after a 2026-11-21 audit because the
old table still described P2#10 as rejected and Phase 7 as not started.

### 2026-11-24 Phase 1 parallel authorization

Director authorized T1.2/T1.3 (TCC batch + permit Tx1 + resume) to proceed in
parallel with the AC-40 zero after-sales amendment. This is a scoped waiver of
the "any contract gap stops implementation" rule for this isolated gap. T1.0
Order-side work is complete; AC-40 end-to-end remains blocked until the PO/
Architect amendment is approved. No AC-VERIFIED or Release PASS.

## 2026-11-24 human decisions: constitution v1.2.0

- Director approved constitution v1.2.0 and set the hierarchy:
  **Constitution > AGENTS.md > docs/collab/CONVENTIONS.md**.
- PAY-SETTLE-01 human decision B: the existing WIP stays on `main` but is
  **frozen**: no merge to release, no publication, no expansion. Open items:
  P-01 safety net only (TCC per-group admission/Σ still missing), P-04, P-05,
  P-06; AC-39…61 remain deferred debts.
- Ratification gate work: G1 AGENTS/CONVENTIONS hierarchy aligned; G2 G-1
  change_id/Actor/Reviewed-by + G-1 check added; G3 ac-coverage columns added;
  G4 this record. G5 human approval recorded here.
- 2026-11-24: Director approved the PAY-SETTLE-01 remediation Plan and
  decisions D-1..D-5 (audited release, 24h legacy terminal states, AfterSales
  freight-responsibility owner, `order.settled` shadow/default false,
  `order.refund_requested` observe-only). Phase 0 contract authoring only; no
  implementation approval.
- Contract debt: SPEC-0001 AC-27 使用 Given/Then/And，下一次 SPEC-0001 版本升级 MUST
  改为 EARS 并登记基线迁移；在此之前不得把 AC-27 标为 AC-VERIFIED。
- 2026-11-24: C-04 proto/config and C-05 migrations passed QC-16 and merged to main
  (`merge: C-04/C-05 contract and migrations`). Phase 0 exit; Phase 1 implementation
  may start on a WIP branch. No service code, AC-VERIFIED, or Release PASS.
- 2026-11-24: Director approved D-22..D-26 defaults:
  - D-22: 24h terminal — no capture/zero -> `EXPIRED_ZERO`; captured and no shipped group ->
    `EXPIRED_REFUND` (does not write `CANCELLED`); shipped group -> `EXPIRED_FINANCE`;
  - D-23: `EXPIRED_FINANCE` only finance/authorized role; no reopen; max 7 days;
  - D-24: authorized role; >=1000 CNY dual review; full audit;
  - D-25: `RELEASED` quota reusable for the same order/group; new refund still needs a new
    application + admission;
  - D-26: 168h starts at the group's first `UNDETERMINED` for that after-sale; other groups/
    later applications do not reset.
  These are business decisions only; SPEC-0002 v1.8, SPEC-0001 v1.4 and
  ADR-0007 Amendment 1 were approved on 2026-11-24 (see the next section).
  D-27..D-29 are closed by ADR-0007 Amendment 1.

## 2026-11-24 SPEC/ADR approvals: SPEC-0002 v1.8 / SPEC-0001 v1.4 / ADR-0007 Amendment 1

Director approved on 2026-11-24, and PO applied the approved drafts to canonical
`docs/specs/`:

- **SPEC-0002 v1.8** — payment settlement & fulfillment admission. Approved
  `docs/specs/0002-payment-settlement-admission.md`; adds S15…S18, INV-45…49,
  AC-40 narrowing, AC-62…66, E22…E31, D-22…D-29. Historical v1.7 text is retained
  in the change log.
- **SPEC-0001 v1.4** — independent approval for the AC-27 EARS baseline migration
  only (`refund_state=REFUNDED`; in-progress `REFUNDING` is not included).
- **ADR-0007 Amendment 1** — accepted 2026-11-24 (A-01…A-14); it maps AC-62…66
  and closes D-27…D-29. The ADR file's own status header is Architect-owned and
  may lag this human-approval record; Architect must sync it.

**Scope of this record**: it is a **contract approval only**, not AC-VERIFIED,
not implementation PASS, and not Release PASS. No code/test evidence is claimed.
AC-39…61 remain deferred; AC-62…66 are newly registered as deferred
(`Developer`, expiry `2026-12-31`, `developer-reviewer/T-002`). Any
implementation/release still requires Reviewer verdict, AC evidence, affected
baseline regression (AC-14/21/25/27/31/32), and human `RELEASE_APPROVED`.

### 2026-11-24 zero after-sale amendment: SPEC-0002 v1.9

Director approved the zero after-sale amendment on 2026-11-24; PO applied it to
`docs/specs/0002-payment-settlement-admission.md` **v1.9 approved**.

- Formalized **S19** (zero refund execution, `refund_applications(source=system_zero)`),
  **INV-50**, and **AC-67/AC-68**; AC-55 gains a formal zero-only exception
  (`pay_cent=0` and no captured intent) while positive refunds keep Payment-fact
  priority unchanged.
- Zero after-sale uses `AdmitRefund` zero short-circuit: same-transaction S19
  `DONE`, no `PENDING`; AfterSales final `REFUNDED/0/CONSUMED/refund_no=''`.
- QC-18: `READY FOR HUMAN ZERO AMENDMENT APPROVAL` (0 BLOCKER).

**Scope**: contract approval only, not AC-VERIFIED, not implementation PASS, not
Release PASS. AC-67/68 are registered as deferred for Developer
(expiry 2026-12-31, `developer-reviewer/T-002`); implementation/C-05 remain
separate gates.

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
| #38 AI mesh | done | explicit read ALLOW plus explicit write DENY. Live UAT: ai-sa ListProducts succeeds, Catalog CreateProduct returns PermissionDenied; `scripts/check-ai-mesh.sh`. |
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

## P3-0 contract prerequisites

| Item | Status | Evidence |
|---|---|---|
| Event registry / outbox validation | done | `internal/platform/events/registry.go`; unknown/malformed payloads fail closed in `outbox.Append`; `TestRegistryCoversKnownEventTypes` |
| Contract gate | done | `make contract-check`: proto guard, event tests, admin/gateway route parity, generated TS freshness |
| Production backup scope | decided | [ADR 0004](adr/0004-p3-0-contract-and-production-scope.md) |

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

## 2026-11-24 PAY-SETTLE-01 implementation review: FAIL

Reviewer scoped review of `d38f282..03a0717` (report
`docs/collab/qc-reports/PAY-SETTLE-01-Review-01.md`) returned **FAIL**,
release_status FAIL. Six contract-level blockers:

1. TCC compensation bypasses group refund admission; refund and shipping can
   both proceed.
2. Shortage groups can still ship (no permit revoke / no shortage or readiness
   check in AuthorizeShipment).
3. Zero-amount refund/shortage path is blocked by `captured > 0`.
4. Historical PAID orders without `settlement_payment_no` cannot refund.
5. Commitments have no RELEASED path; quota is permanently reserved.
6. Group quota does not filter freight by responsibility.

No `[ARCHITECTURAL_FLAW]`: ADR-0007 D-08/D-09/D-10 and §6.1 already specify the
required behavior; these are implementation deviations. The goal remains active;
P0-A/P0-B is **not complete** and no Release PASS may be claimed.

### 2026-11-24 remediation after Review-01

At head `6550a2c`: P-01 safety net (TCC cancel blocks all groups; full fan-out
admission still missing), P-02 fixed (AuthorizeShipment checks readiness, group
refund state and open shortage), P-03 fixed (zero-amount admission). P-04
(historical PAID resolution), P-05 (RELEASED) and P-06 (freight responsibility)
remain open. Review-01 remains FAIL; no Release PASS until a fresh scoped review
at a new head.

## 2026-11-23 human decisions: payment settlement & fulfillment admission (P0)

Source: external review `docs/ext-review-1123.md`; classification and evidence in
`docs/ext-review-1123-triage.md`. Human approved the H1–H6 recommendations on
2026-11-23. This section records status and pointers; normative text lives in
Spec/ADR/CONVENTIONS.

| # | Decision | Status |
|---|---|---|
| H1 | First payment must be exact (`captured == pay_cent`), one active payment per order, duplicate capture refunded, `pay_cent=0` exempt, **no partial payment this phase** | approved; SPEC-0002 v1.7 approved; ADR-0007 accepted 2026-11-23; implementation next |
| H2 | Payment-settlement + fulfillment-admission is a P0 project (`change_id=PAY-SETTLE-01`), split P0-A funds / P0-B admission / P0-C three deterministic release errors; production switches stay false; UAT split may stay but is **not** release evidence | approved; freeze in effect |
| H3 | Integration owner: **Director** (name provided 2026-11-23); Architect acts as design owner/secretary; one payment/order Developer as deputy; no per-service split ownership | approved; Director named, deputy TBD |
| H4 | Withdraw the `c-warehouse-review` “functionally complete” release implication, keep history | approved; doc amended 2026-11-23 |
| H5 | Mesh narrowing now (security, not an AI feature); production `ai.enabled=false` until live Mesh UAT; AI-0 design may follow P0, AI-1…5 deferred | approved; static config changed, live UAT pending |
| H6 | Approve `INV:` in reviewer metadata / `baseline_impact` (G1) and the QC crash-window checklist (G2); no new machine gates | approved; CONVENTIONS §2.2/§3/§4.1 and `prompt/Quality-Checker.md` updated |

**Freeze / release hold**: no Release PASS may cite `make gates-strict`, thread
CLOSED, AC-REF counts, or the historical C/UAT results. Unfreeze only when the
§7.3 matrix in the triage doc, G-4, explicit `--release --range`, and the
five-party final-state check all pass.

**Next gate**: ADR-0007 v0.4 is **accepted** (Director 2026-11-23); SPEC-0002 is
v1.7 (INV-43 scope clarified). P0-A/P0-B implementation starts with the C-1…C-9
preconditions: `CheckOrderReady`/`CancelIfCreated`/`GetIntent(intents[])` proto
contracts, event registry, worker assembly, migration review, and the Inventory
CONFIRMED-release check. Until those contracts land, keep the production flags
false and do not claim AC-VERIFIED.

**C-1/C-8/C-9 closed**: Director approved 2026-11-23; SPEC-0001 v1.3 and
SPEC-0002 v1.6 approved; AC-39…61 registered as deferrals
(developer-reviewer/T-002, expiry 2026-12-31); commits `9dc38b6`, `6aa12f3`.

**ADR-0007 handoff (R-1…R-4, from QC-08)**: (R-1) Σ group caps ≠ order remaining
captured needs an owner/action for manual adjudication; (R-2) choose accounting
for order-level cancellation refunds vs group caps; (R-3) historical no-group
orders use the whole remaining amount for G1; (R-4) TCC refunds freight first, so
a later responsibility ruling must not double-refund (shared group ledger).

**2026-11-23 approvals and landing**: Director approved C-1 (SPEC-0001 v1.3),
C-8 (D-11…D-21 all defaults), and C-9 (ledger + move + commit). SPEC-0001 v1.3
and SPEC-0002 v1.6 are approved; AC-39…61 are registered in
`ac-baseline.txt`/`ac-coverage.tsv` as deferrals (developer-reviewer/T-002,
expiry 2026-12-31). ADR-0007 is the next gate.

**2026-11-23 0012 decision**: long-term split chain (not the operator bridge).
Audit: dev `mall-data` and UAT `mall-uat-data` applied only up to
`aftersales/0011_group_items.sql`; no 0012 row/attempt. Local compose has no
`schema_migrations` and no `line_no`. So 0012 can be replaced before any
persistent environment applies it; implementation still needs Architect sign-off,
a backfill tool and a disposable-DB rehearsal.

**Gate note (historical, 2026-11-23; superseded by the 2026-11-24 approvals
above)**: while the Spec was still unapproved, the draft lived in
`docs/po/drafts/` and was not indexed in `docs/specs/README.md` or `Spec.md`. On `SPEC_APPROVED`,
Reviewer adds AC-39…61 to `docs/collab/ac-baseline.txt` and `ac-coverage.tsv`
(owner + expiry + thread), the PO moves the file into `docs/specs/`, updates both
indexes, and the approval is committed as `docs(spec): SPEC-0002 v1.x ...` in the
same round. Do not index or commit the draft before that sync.

P0-C (order list scan, aftersales migration ordering, Helm flag plumbing) may
start independently because it implements already-approved contracts.
