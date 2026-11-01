# File ownership (parallel implementation)

To avoid merge conflicts, each agent only edits its own paths.

## Platform owner (main agent)

- `go.mod`, `go.sum`
- `Makefile`, `buf.gen.yaml`, `.github/**`
- `internal/platform/**`
- `internal/contracts/**`
- `api/proto/**`, `api/gen/**`
- `deploy/compose/infra.yml`, root compose files
- root `README.md`, `docs/contracts.md`, `docs/ownership.md`, `docs/dtm.md`

## Service owners

Each service agent owns:

- `cmd/<service>/**`
- `internal/services/<service>/**`
- `migrations/<service>/**`
- `deploy/compose/services/<service>.yml`
- `docs/services/<service>.md`

No service agent edits `go.mod`, `internal/platform`, `api/proto`, or another service.

## Gateway/admin owner

- `cmd/gateway/**`, `cmd/admin/**`
- `internal/services/gateway/**`, `internal/services/admin/**`
- `api/openapi/**`

## Infra owner

- `deploy/**` except `infra.yml` owned by platform
- `docs/ops/**`
- observability configs

## Contract changes

If a service needs a proto/event/schema change, report it to the main agent.
The main agent updates the contract, regenerates code, and broadcasts the change.
