# Mall frontend

Monorepo for the mall web applications.

```text
apps/admin-web       # B-end, Vue 3 + Vite + Element Plus
apps/storefront-web  # C-end, Nuxt 3 + Vant
packages/api         # typed HTTP client for gateway/admin BFFs
packages/utils       # money/date helpers
packages/design-tokens
```

## Development

```bash
cd frontend
pnpm install
pnpm dev:admin       # http://127.0.0.1:5173/admin/
pnpm --filter @mall/storefront-web dev # http://127.0.0.1:3000/
```

The admin Vite server proxies `/api/admin` to `:18081` and `/api/v1` to
`:18080`. Start the Kubernetes port-forwards first:

```bash
kubectl -n mall port-forward svc/gateway 18080:8080
kubectl -n mall port-forward svc/admin 18081:8081
```

Override the dev targets with `VITE_DEV_GATEWAY_URL` and `VITE_DEV_ADMIN_URL`.
The storefront dev server proxies `/api` to the gateway; SSR uses
`NUXT_PUBLIC_SSR_API_BASE` (default `http://127.0.0.1:18080`).

## Checks

```bash
pnpm typecheck
pnpm test
pnpm build
```

## Images

```bash
docker build -f apps/admin-web/Dockerfile -t mall/admin-web:dev .
docker build -f apps/storefront-web/Dockerfile -t mall/storefront-web:dev .
```

The Helm chart serves admin-web at `/admin` and storefront-web at `/`; both
apps call `/api` on the same origin. `NUXT_PUBLIC_SSR_API_BASE` must point at the
in-cluster gateway for server-side rendering.

## Scope

This is the initial admin/storefront foundation. The admin app currently covers
login, catalog, inventory, orders, shipments, after-sales and promotion
campaign/rule editing plus the promotion simulator. The storefront covers
product browse/detail, login, cart, checkout preview/place/mock pay, orders and
after-sales lists. Advanced storefront features, RBAC and a generated OpenAPI
client are planned follow-ups; the API package is the single place to update
when BFF contracts change.
