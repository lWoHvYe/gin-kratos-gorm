# identity service

gRPC port `9001`, health `9101`, database `mall_identity`.

## RPCs

- `Register(username, password, nickname)` → access + refresh token.
- `Login(username, password)` → access + refresh token.
- `Refresh(refresh_token)` → revokes the presented refresh token and issues a
  new access/refresh pair (rotation). Reuse of a revoked token is rejected.
- `Logout(refresh_token)` → revokes the token; idempotent.
- `GetUser(user_id)` → profile and roles.
- `CreateAddress` / `ListAddresses`.

Access tokens are HS256 JWTs (`JWT_SECRET`, `JWT_ISSUER`, `JWT_ACCESS_TTL`);
refresh tokens are opaque random values stored only as SHA-256 hashes in
`refresh_tokens`.

The gateway exposes the public auth routes under `/api/v1/auth/*`.
