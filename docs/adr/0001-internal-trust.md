# ADR 0001: internal service trust model

Status: accepted (2026-11)
Scope: how services authenticate callers inside the cluster.

## Context

The BFF validates the user's JWT; every gRPC service trusts the `user_id`,
`admin` and similar fields carried in the request. There is no application-level
authentication between services. That was acceptable for a single-tenant demo
but it is a real trust boundary decision, so it is recorded here.

Two options were considered:

1. **Propagate a signed user context.** The BFF mints a short-lived, signed
   context (uid, roles, request id) in gRPC metadata; a server interceptor
   validates it and business code reads identity from the context instead of the
   request body.
2. **Trust the mesh.** Authenticate the *workload*, not the end user: rely on
   Istio STRICT mTLS, AuthorizationPolicy (source ServiceAccount + path) and
   NetworkPolicy. User identity is validated once at the BFF and forwarded.

## Decision

Adopt option 2 for the current single-cluster, single-tenant deployment, with the
boundaries and residual risks below. Option 1 remains the upgrade path once the
triggers in "Revisit when" are met.

## What actually enforces it

- **mTLS**: `PeerAuthentication` is STRICT in the `mall` namespace.
- **Workload identity**: `AuthorizationPolicy` rules allow a source
  ServiceAccount to call a target service's paths; L4 policies restrict the
  ports reachable from the ingress gateway and waypoint.
- **Network**: `NetworkPolicy` default-deny, with same-namespace and
  `mall-data` exceptions. Any pod in `mall` can reach any service port.
- **User identity**: validated only by the gateway/admin BFFs. Services accept
  `user_id` / `admin` fields as-is. This is documented in `cart`:

  > The service is intentionally free of authentication: user_id is supplied by
  > the gateway in every request.

## Residual risks (accepted)

1. A compromised pod in `mall` can impersonate any user by calling a service
   directly with an arbitrary `user_id`, or act as admin by setting
   `admin: true` on admin-capable RPCs.
2. AuthorizationPolicy paths are service-prefixed (`/mall.v1.OrderService/*`),
   not method-scoped, so a compromised payment/seckill/dtm pod can call any
   order method, including `MarkPaid` and the TCC branches.
3. Compose (local development) has no mesh and no L7 policy: anything on the
   Docker network can call the gRPC ports. Treat Compose as a trusted network
   only.
4. There is no audit trail of who performed privileged operations beyond the
   request log line (an admin write audit middleware now covers the BFF).

## Compensating controls

- Keep the service-list authorization rules narrow: add a rule only for a
  service that must call another; do not add namespace-wide allow.
- BFF write operations are audit-logged (actor id, method, path, request id).
- Secrets fail closed outside development (`JWT_SECRET`,
  `SECKILL_TOKEN_SECRET`, `PAYMENT_SECRET`), so a stolen manifest does not give
  a usable token-signing key.
- The admin surface requires the `admin` role at the BFF and is not exposed
  outside the gateway.
- Pod security: non-root, read-only root filesystem, no service-account token
  mounted.

## Revisit when (move to option 1)

- Any gRPC port becomes reachable from outside the cluster, or a second tenant
  shares the deployment.
- A compliance requirement needs per-user authorization at the service layer.
- A workload is considered untrusted (third-party workload, shared namespace).
- We add a non-HTTP caller that cannot be constrained by the mesh
  (for example an external batch job).

Migration sketch for option 1: BFF signs `{uid, roles, exp, jti}` with a
dedicated internal key, sends it as gRPC metadata, a `grpcx` interceptor
validates it into the context, services read the uid from context, system
callers (DTM, workers) get a service principal with explicit scopes, and the
request-level `user_id` fields are deprecated one service at a time.

## Consequences

- No service-level user checks today: any new service must document that its
  caller is expected to pass a validated `user_id`.
- Method-level AuthorizationPolicy is a follow-up that must be tested in
  staging; a wrong DENY can break checkout, so it is not enabled blind here.
