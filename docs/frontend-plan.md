# Frontend plan

This document plans the web frontends for the mall. It is intentionally a
separate frontend workspace inside this repository; the Go services and the
frontend applications keep independent build, test and deployment lifecycles.

## Recommendation: frontend first, B-end first

Recommended order:

```text
1. Frontend foundation
2. B-end admin MVP
3. C-end storefront MVP
4. Seckill backend + seckill frontend module
```

If the choice is strictly "frontend first or seckill first", choose
**frontend first**. The promotion engine is now usable through APIs, but an
operator cannot safely configure campaigns, inspect budgets, simulate stacking
or handle orders/returns without a back office. A customer-facing storefront is
also required before seckill traffic has a meaningful destination.

Seckill backend work can start in parallel **after the B-end promotion UI is
usable**, but seckill should not be the first frontend or the first product
surface.

## Projects

Start with two applications rather than one role-switched SPA:

| App | Audience | Default technology |
|---|---|---|
| `apps/admin-web` | internal operations, desktop | Vue 3 + TypeScript + Vite + Element Plus |
| `apps/storefront-web` | customers, mobile-first public web | Nuxt 3 (Vue 3) + Vant 4 + TypeScript |

Possible later apps, not part of the first milestone:

- `apps/merchant-web`: only if multi-merchant/supplier workflows are added;
  the current backend is single-operator and does not justify it yet.
- `apps/ops-web`: only if operations/risk/finance needs diverge from the admin
  console.
- a mini-program: separate channel and UI stack (uni-app/Taro), sharing the
  generated API client and types but not the Vue components.

Do not put B-end and C-end in one SPA. They have different authentication UX,
route guards, bundles, design systems, release cadence and failure domains.

## Workspace layout

```text
frontend/
  package.json
  pnpm-workspace.yaml
  turbo.json
  tsconfig.base.json
  .npmrc
  apps/
    admin-web/
    storefront-web/
  packages/
    api/                 # generated OpenAPI types + fetch client
    contracts/           # optional generated request/response schemas
    design-tokens/       # colors, spacing, typography, z-index
    utils/               # money, dates, validation helpers
    eslint-config/
    tsconfig/
```

Use **pnpm workspaces + Turborepo**. It is mature, fast, and small enough for
two applications; Nx would be overkill at this stage.

## Mature framework choices

### B-end

- Vue 3, `<script setup>`, TypeScript strict
- Vite
- Vue Router
- Pinia for session/UI state
- TanStack Vue Query for server state/caching
- Element Plus
- VeeValidate + Zod for forms
- VueUse
- ECharts for dashboards
- Vitest + Vue Test Utils, Playwright

### C-end

- Nuxt 3 (Vue 3), TypeScript strict
- Vant 4 mobile components
- Pinia + TanStack Vue Query
- VeeValidate + Zod
- VueUse
- Nuxt image/route rules where appropriate
- Vitest + Playwright

Nuxt is recommended for the public storefront because product/search pages need
SEO, social previews and fast first content. If the C-end is strictly an
in-app H5 with no SEO requirement, Vite + Vue 3 + Vant is a valid lighter
alternative. Make this decision before scaffolding the storefront.

## API contract and generated client

Do not hand-maintain duplicate request/response interfaces for every page.

1. Add an OpenAPI document for the gateway and admin HTTP BFFs
   (`docs/openapi/gateway.yaml`, `docs/openapi/admin.yaml`), or one combined
   document with tags.
2. Generate TypeScript types with `openapi-typescript` and use
   `openapi-fetch`, or generate Vue Query hooks with `orval`.
3. Keep `packages/api` as the only place that knows URL paths, auth headers,
   error envelopes and refresh behavior.
4. Add contract checks to CI so frontend code cannot silently drift from the
   BFF.

Important frontend contract rules:

- **int64 identifiers**: keep IDs as strings in the client contract where the
  backend may emit values beyond JavaScript's safe integer range. Do not parse
  all IDs as `number`.
- Money is integer cents. Never use floating point arithmetic for totals,
  discounts or allocation; use integer helpers in `packages/utils`.
- All API errors follow the BFF envelope
  (`code`, `message`, `request_id`); map them to typed UI errors.
- Use Unix seconds for timestamps and format in the browser timezone.

## Authentication

- Access tokens stay in memory or a short-lived storage suitable for the
  chosen threat model; do not put long-lived refresh tokens in `localStorage`.
- Recommended backend follow-up: set the refresh token as a `HttpOnly`,
  `Secure`, `SameSite` cookie and keep access tokens short-lived.
- B-end route guard requires the `admin` role.
- C-end route guard redirects to login and preserves the return URL.
- Centralized 401 handling: one refresh attempt, then logout and redirect.
- Never trust client-side role checks for authorization; the admin BFF remains
  the authority.

## B-end functional scope

### Milestone B1: shell and operations

- Login/logout/session refresh
- Admin layout, navigation, route guards
- Error/empty/loading states
- Category/brand list
- Product list, detail, draft/publish/off-sale
- SKU list and status
- Inventory list and restock/adjust
- Order list/detail, cancel
- Shipment list/detail, ship
- After-sales list, approve, receive, refund

### Milestone B2: promotion operations

- Campaign list/editor: type, time window, status, priority, budget,
  per-user limit, stack mode
- Promotion rule editor:
  - scope picker (all/SKU/SPU/category/brand)
  - condition builder for common conditions
  - advanced `condition_expr` editor with backend validation
  - effect editor (fixed/percent/threshold/free shipping)
  - budget, per-user limit, exclusive, stack group
- Publish/pause/end actions
- Promotion simulator (`POST /promotions/evaluate`): applied, rejected reasons,
  discount breakdown, budget state
- Reservation/budget ledger view
- Order price breakdown including promotion and coupon discounts

The current condition expression is intentionally powerful. The UI should offer
a structured builder for common cases and an advanced expression textarea as a
fallback, not force operators to write expressions for a simple full-reduction.

### Milestone B3: after-sales and audit

- Return tracking visibility
- Receive-with-condition (`SELLABLE`/`DAMAGED`)
- Refund result and refund number
- Promotion budget/ledger history
- Permission granularity later; the current backend only has an `admin` role,
  so do not invent a frontend RBAC model before backend support exists.

## C-end functional scope

### Milestone C1: browse and buy

- Home/landing
- Product list/search with category, brand, price and sort filters
- Product detail with images/attributes/SKU selection
- Cart add/update/remove/merge and stock feedback
- Address book
- Checkout preview with promotion/coupon breakdown
- Place order
- Mock payment callback
- Order list/detail, cancel, receive

### Milestone C2: after-sales

- Apply for refund-only/return-refund
- Return tracking submission
- After-sales list/detail
- Refund status and amounts

### Milestone C3: promotion presentation

- Show applied promotions on product/quote/order
- Coupon selection
- Discount breakdown per line and order
- Countdown/labels only where backend supports the activity type
- Explain rejected promotions only in debug/admin views, not noisy customer UI

## Seckill frontend

Seckill is a separate C-end experience, not a normal product-list filter:

- activity landing page
- countdown and sold-out state
- one-unit/per-user limit messaging
- queue/waiting state
- token/result state
- non-blocking retry when the queue is full
- order result and payment handoff

It must not poll a general product endpoint at high frequency. The backend
should provide an activity stock/token API with a clear state machine; the
frontend should use short polling only for queue/result state, or a server push
if the infrastructure later supports it.

Do not build the seckill UI before:
1. the promotion activity/rule admin UI is usable;
2. C-end login/cart/order/payment shell exists;
3. backend seckill stock/token semantics are fixed.

## Deployment

Each app is a separate static/SSR workload:

- `admin-web`: Nginx static image, SPA fallback to `index.html`.
- `storefront-web`: Nuxt Node server or prerendered/SSR deployment depending on
  the final hosting choice.
- Separate Kubernetes Deployments/Services and HTTPRoute rules:
  - `/admin` -> admin-web
  - `/` -> storefront-web
  - `/api` -> gateway/admin BFF
- Same-origin deployment is preferred to avoid CORS and cookie complexity.
- Runtime config should be injected (not baked into every image) for API base
  URL, environment and feature flags.
- Add CSP, cache headers for immutable assets, and no-cache for HTML.

Near-term backend/frontend prerequisites:

- OpenAPI contract for the BFFs
- CORS/allowlist only if deployments are cross-origin
- HttpOnly refresh cookie
- image upload/object storage; current catalog accepts image URLs only
- stable error envelope and pagination conventions
- rate limiting for public storefront endpoints

## Testing and quality

- TypeScript strict mode
- ESLint + Prettier + Stylelint (or the Vue ecosystem equivalent)
- Vitest component/unit tests
- Playwright against a running Compose/K8s stack
- MSW fixtures for frontend-only development
- accessibility checks (labels, keyboard, focus)
- performance budgets: route-level code splitting, image lazy loading,
  storefront Lighthouse budget
- CI jobs: install, lint, typecheck, unit, build, Playwright smoke, OpenAPI
  contract check
- Keep frontend CI independent from backend image publishing, but gate on the
  API contract.

## Delivery phases

```text
F0  workspace, design tokens, generated API client, auth shell, CI
F1  B-end catalog/orders/inventory/after-sales MVP
F2  B-end promotion/campaign editor + simulator
F3  C-end browse/product/cart/checkout/orders MVP
F4  C-end after-sales + promotion presentation
F5  seckill activity frontend
```

Recommended staffing split if the team is large enough:

- shared platform/API/design system: 1-2 engineers
- B-end: 2 engineers
- C-end: 2-3 engineers
- QA/design: shared

If the team is smaller, do B-end and C-end sequentially and keep the shared
packages small.

## Decisions needed before scaffolding

1. **C-end channel**: public SEO/H5 (Nuxt 3 + SSR) or in-app H5 (Vite SPA)?
2. **B-end UI language/design system**: Element Plus is the default; confirm
   brand/design tokens.
3. **Same-origin routing**: `/` storefront, `/admin` back office, `/api` BFF?
4. **Auth transport**: are we willing to change the BFF to HttpOnly refresh
   cookies before frontend work?
5. **Image upload**: keep URL-only for MVP or add object storage now?
6. **Seckill timing**: business deadline; if urgent, backend seckill can run in
   parallel after B-end promotion UI, but not before the C-end shell.
