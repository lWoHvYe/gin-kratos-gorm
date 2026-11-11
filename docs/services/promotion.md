# Promotion engine

Phase 2 adds a promotion engine inside the pricing process and database, exposed
through the separate `PromotionService` gRPC API. Keeping it co-located makes
quote creation and budget reservation share one transaction boundary; the
service boundary lets it be split into its own deployment later without
changing order/admin clients.

## Model

- **Campaign** is the activity container: code, type (`NORMAL`, `MEMBER`,
  `SECKILL` reserved for future execution), time window, priority, budget,
  per-user limit and stack mode.
- **PromotionRule** is a condition + effect attached to a campaign:
  - `scope_json`: `{"all":true}` or SKU/SPU/CATEGORY/BRAND ids.
  - `condition_expr`: safe `expr-lang/expr` expression over
    `subtotal_cent`, `cart_total_cent`, `quantity`, `item_count`, `user_id`,
    `coupon_applied`, `now_unix`.
  - `effect_json`: `FIXED_AMOUNT`, `PERCENT`, `THRESHOLD` or
    `FREE_SHIPPING`.
  - priority, exclusive, stack_group, max discount, rule/campaign budget,
    coupon compatibility.
- **Reservations** use `promotion_reservations`, `promotion_reservation_items`
  and `promotion_budget_ledger`. A quote snapshots applied promotions; order
  creation reserves the exact snapshot by quote id, so the client cannot
  tamper with the discount.

## Stacking

Rules are sorted by priority/id. `exclusive` stops further stacking,
`stack_group` is mutually exclusive within the group, and a hard cap limits the
number of selected rules. The MVP picks the priority order; the response
contains applied and rejected candidates for explainability. A best-combination
optimizer can be added behind the same evaluation contract.

## Budget lifecycle

- `ReservePromotions(quote_id, order_no)` checks rule/campaign budgets and
  per-user limits under `SELECT ... FOR UPDATE`, then increments `reserved`.
- `ConfirmPromotions(order_no)` moves HELD to CONFIRMED and reserved to consumed;
  it is called after payment and retried from `shop.order.paid.v1`.
- `ReleasePromotions(order_no)` moves HELD to RELEASED; cancellation and
  timeout release both budget reservations and the coupon.
- A budget of `0` means unlimited. Amounts are integer cents.

## Coupon interaction

Coupons remain in pricing. Quote evaluation passes `coupon_applied`; a rule
with `allow_with_coupon=false` is rejected. The order now locks the coupon
before the create saga, consumes it after payment, and unlocks it on cancel or
timeout. This closes the pre-Phase-2 gap where `ConsumeCoupon` ran without a
matching lock.

## Seckill extension

`Campaign.type` and the evaluation boundary leave room for a seckill execution
adapter, but this phase does not implement activity stock, Redis/Lua deduction,
tokens, queues or anti-bot. Those belong to a dedicated execution path; the
promotion engine remains the eligibility/price/budget source.

## Ports

The `PromotionService` is registered on the pricing gRPC server (`9005`) and
uses the `mall_pricing` schema. No separate deployment is required in Phase 2.
