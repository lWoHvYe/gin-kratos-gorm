# etcd configuration center

The project now supports an optional etcd overlay while keeping environment
variables as the bootstrap and highest-priority source.

## Precedence

```text
compiled defaults
  < etcd YAML/JSON overlay
  < environment variables
```

`ETCD_ENDPOINTS` and `CONFIG_ETCD_PATH` are bootstrap values and must come from
the environment; they cannot themselves be stored in etcd.

## Enable it

```bash
export ETCD_ENDPOINTS=127.0.0.1:2379
export CONFIG_ETCD_PATH=/mall/config/dev/order.yaml
# optional: treat the path as an etcd prefix
export CONFIG_ETCD_PREFIX=false
go run ./cmd/order
```

Example key:

```yaml
# etcdctl put /mall/config/dev/order.yaml "$(cat order.yaml)"
grpc_addr: ":9006"
http_addr: ":9106"
mysql_dsn: "root:root@tcp(127.0.0.1:3306)/mall_order?parseTime=true&loc=Local&charset=utf8mb4"
nats_url: "nats://127.0.0.1:4222"
dtm_address: "127.0.0.1:36790"
jwt_issuer: "mall"
jwt_access_ttl: "15m"
jwt_refresh_ttl: "720h"
```

`JWT_SECRET` is intentionally **not** read from the etcd YAML in this version;
keep it in a secret store/environment.

The overlay is applied through Kratos v3's
`contrib/config/etcd/v3` source and `config.Config.Scan`. If the key is missing
or etcd is unavailable while `CONFIG_ETCD_PATH` is set, startup fails rather
than silently using a partial config.

## Dynamic updates

`LoadEtcd` returns an `EtcdProvider` with:

- `Scan` — read the current document
- `Watch` — subscribe through Kratos `config.Observer`
- `Close` — stop watchers and close the etcd client

`Load` currently closes the provider after applying the startup overlay. Future
dynamic fields should keep the provider alive in the composition root and watch
only explicitly safe values (feature flags, timeouts, rate limits), not DSNs,
listen ports, or secrets.

## Security and operations

- Store only non-secret dynamic configuration in etcd.
- Use TLS/auth for etcd in production.
- Keep one key per environment/service, for example:
  `/mall/{env}/{service}.yaml`.
- Validate the whole document before applying it; do not partially accept a
  malformed YAML.
- Keep a version/audit trail outside etcd (GitOps or a config service).
- Treat configuration as a deployment artifact: review, test, roll back.
