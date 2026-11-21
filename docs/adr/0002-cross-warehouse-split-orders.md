# ADR 0002: cross-warehouse split orders (warehouse mode C)

- Status: Proposed — awaits human approval before schema/business-code changes
- Date: 2026-11-21
- Supersedes: none
- Related: `docs/ext-review.md` section 4 (multi-warehouse decision), A/B settlement ADRs

## Context

The current order path is single-warehouse:

- `orders` has no warehouse/group concept.
- `order_items` has no `warehouse_id`.
- `inventory_reservations` is unique by `(order_no, sku_id)`, so one SKU cannot
  be reserved in two warehouses.
- `shipments` is unique by `order_no`, so one order cannot have multiple
  parcels.
- `order_shipping_permits` is keyed by `order_no`.
- After-sales restock uses a per-SKU task and the default warehouse.

The product decision is **C: support cross-warehouse split orders**. This is a
new order/fulfillment product version, not a parameter change. It must not be
implemented by changing `defaultWarehouseID` in call sites.

## Decision

### 1. New aggregate: order fulfillment group

Add `order_fulfillment_groups`:

```text
group_no            deterministic (<order_no>:<seq> or <order_no>:<warehouse>)
order_no
warehouse_id
status              PENDING | ALLOCATED | SHIPPED | RECEIVED | CANCELLED
freight_cent
reservation_state   NONE | HELD | CONFIRMED | RELEASED
version, created_at, updated_at
UNIQUE (order_no, group_no)
UNIQUE (order_no, seq)
```

- Order owns the allocated group snapshot.
- Inventory owns real stock; it must not be asked to infer the order's groups.
- Fulfillment owns one shipment per group.
- AfterSales owns the return/restock task, but the item's origin
  `warehouse_id`/`group_no` is copied from the order line.

### 2. Order lines become allocation lines

`order_items` gains:

```text
line_no              deterministic per order
warehouse_id
fulfillment_group_no
```

- `UNIQUE (order_no, line_no)`; `(order_no, sku_id)` is no longer unique.
- A SKU may appear on multiple line numbers in different groups.
- **v1 scope:** allocation is line-level. If a requested quantity cannot be
  served by one warehouse, the checkout splits that SKU into two order lines
  before payment. Splitting a single line's quantity dynamically after payment
  is out of scope.
- Inventory reservations become `UNIQUE (order_no, sku_id, warehouse_id)` and
  carry `line_no` so two groups cannot collide.
- Inventory whole-order operations (B) extend the item key from `sku_id` to
  `(sku_id, warehouse_id)`; the operation `scope_hash` includes both. Give the
  new protocol an explicit id version (`inv:v2:<type>:...`) so old operation IDs
  remain replayable during the dual-read window.

### 3. Allocation and freight happen before payment

The current BFF `checkout/preview` flow prices a cart without warehouse
knowledge. C needs a stable allocation before the customer pays, otherwise the
freight total can change after payment.

Proposed flow:

1. `Checkout preview` calls Inventory `AllocateOrder` (new read-only RPC) with
   `(sku, quantity, address/region)`. It returns per-line warehouse
   assignments **without reserving stock**.
2. BFF passes those assignments to Pricing `CreateQuote` (add
   `warehouse_id`/`group` to the quote line). Pricing returns per-group freight
   and the order total.
3. BFF displays the split groups and freight before payment.
4. `Place order` passes `quote_id`; Order revalidates the allocation snapshot,
   persists groups/lines, then starts one Reserve action per group/line.
5. At payment confirmation, Inventory confirms per group through the B
   operation protocol.

Allocation is a **quote fact**, not a reservation. A stale allocation is
revalidated at place; if stock changed, the customer gets an explicit
`FAILED_PRECONDITION` and a new preview, never a silent warehouse swap.

Freight is computed per group and summed. The exact freight rule (flat per
warehouse, weight/region, or subsidized) is a product parameter, not hardcoded
in Order/Fulfillment.

### 4. Lifecycle and projections

Keep `orders.status` as the overall lifecycle; do **not** add one status value
per partial combination.

- `orders.status`: `PENDING_PAYMENT -> PAID -> ALLOCATED -> SHIPPED -> COMPLETED`,
  plus `CANCELLED/CLOSED`.
- Extend the existing `orders.shipment_status` projection (do **not** add a
  second `fulfillment_state` field) with:
  `UNSHIPPED | PARTIALLY_SHIPPED | SHIPPED | PARTIALLY_RECEIVED | RECEIVED | CANCELLED`.
- First group ship: lifecycle stays `ALLOCATED`, projection becomes
  `PARTIALLY_SHIPPED`.
- All groups shipped: lifecycle becomes `SHIPPED`, projection `SHIPPED`.
- First receipt: projection `PARTIALLY_RECEIVED`; all received: lifecycle
  `COMPLETED`.
- `available_actions` is derived from the aggregate state:
  `ship_group:<group_no>`, `receive_group:<group_no>`,
  `cancel_unshipped`, `apply_after_sale`, etc. The BFF/frontend never derive
  actions from `status` or a group count.

### 5. Permits, cancellation and refunds

- `order_shipping_permits` is keyed by `(order_no, fulfillment_group_no)`;
  `AuthorizeShipment` takes `group_no` and a `shipment_no`; each group has its
  own permit/CAS.
- Cancelling an order is allowed only while no group is SHIPPED/RECEIVED. For
  the existing unpaid cancel path it releases every unshipped group. Paid,
  unshipped groups are cancelled through refund/after-sales, not through the
  order-cancel state; a dedicated partial-cancel action is an open UX question.
  A paid order with a partial shipment can only be refunded/after-sold by line.
- Refund-only after a group ships remains allowed (A decision). Refund
  admission is still per after-sale item/line and the Order-level permit for
  that group is revoked if the group has not shipped.
- `RETURN_REFUND` restocks to the item's origin `warehouse_id`. A cross-warehouse
  return is an explicit exception flow, not a default.

### 6. Events

- `shop.order.fulfillment_group.allocated.v1`
- `shop.order.fulfillment_group.cancelled.v1`
- `shop.shipment.shipped.v1` / `.delivered.v1` include `group_no` and
  `warehouse_id`
- `shop.order.fulfillment_projected.v1` carries the aggregate projection
- Existing `shop.order.paid/cancelled/refund_projected` remain.

Every event is written by the owning service in the same transaction as its
state change; no consumer infers a group from a SKU.

## Schema/migration outline

1. Create `order_fulfillment_groups`, backfill every existing order with one
   `G1`/warehouse 1 group and copy `freight_cent`; backfill `order_items` with
   `line_no=id`, `warehouse_id=1`, `fulfillment_group_no='G1'`.
2. Change `inventory_reservations` uniqueness to
   `(order_no, sku_id, warehouse_id)`; backfill `line_no` from existing
   reservations (single line).
3. Change `shipments` uniqueness from `(order_no)` to
   `(order_no, fulfillment_group_no)`; backfill existing shipment with `G1`.
4. Backfill `order_shipping_permits` with `group_no='G1'`; change its primary
   key.
5. Extend B operation items with `warehouse_id`; add the v2 operation-id
   version for rebuilt `scope_hash` inputs.
6. Extend `shipment_status` and derive per-group `available_actions`.

All migrations are additive first, then constraint swaps after the read/write
paths are dual-compatible. The feature flag `WAREHOUSE_SPLIT_ENABLED` defaults
false; with it false, allocation always produces one warehouse-1 group and the
old behavior is preserved.

## Rollout phases

1. **Shadow read**: schema + backfill + allocator dry-run; still one group.
2. **Quote/allocation**: `AllocateOrder` + quote groups; compare with single
   warehouse; no split orders accepted yet.
3. **Split reservations**: change reservation key, B operation key; test
   concurrent confirm/cancel per group.
4. **Split shipments/permissions**: group permits, multiple shipments, lifecycle
   projection.
5. **After-sales/restock**: origin warehouse, per-line caps.
6. **BFF/UI**: groups, freight, per-group actions, partial shipment display.

A phase is not “done” without MySQL concurrency/fault tests, a migration
preflight on cloned data, and live dev E2E for split groups.

## Non-goals

- Warehouse-to-warehouse transfer.
- Dynamic re-allocation after payment.
- Split a single order line's quantity across warehouses in v1 (line-level
  split only; see open questions).
- Multi-currency/tax redesign.
- A second inventory ledger or a generic optimizer.

## Open questions (human decision before phase 2)

1. Freight rule: flat per warehouse, weight/region, or subsidized single fee?
2. Allocation policy: nearest warehouse, stock priority, configured priority,
   or manual admin override?
3. Must a single SKU line ever split its quantity across warehouses in v1?
4. Cross-warehouse returns: always origin warehouse, or allow another warehouse
   with a transfer task?
5. Partial cancellation/refund UX: per group or per line?

## Consequences

Positive: real split fulfillment, per-group permits/refunds, no fake
multi-warehouse parameter.

Cost: reservation uniqueness and operation keys change; quote/pricing gains
warehouse input; order/fulfillment/after-sales read models gain groups; more
concurrency combinations. This is why it must be a separate product version,
not a patch on the single-warehouse path.
