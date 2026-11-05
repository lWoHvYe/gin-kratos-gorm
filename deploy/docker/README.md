# Local etcd + DTM simulation

This profile runs **etcd and DTM in Docker** while the Go services run on the
host. DTM must call the host services, and the host services must call DTM.

## 1. Prerequisites

- Docker Desktop (macOS/Windows) or Docker Engine with Compose v2.
- A MySQL that DTM can reach. This compose file does **not** start MySQL.
- Host MySQL must have the `dtm` and `dtm_barrier` schemas.

Apply the schemas once:

```bash
mysql -h127.0.0.1 -uroot -proot < deploy/compose/init/mysql/00-databases.sql
mysql -h127.0.0.1 -uroot -proot < deploy/compose/init/mysql/02-dtm.sql
```

If MySQL is already the `deploy/compose` MySQL container, those schemas are
already applied by its init scripts.

## 2. Configure

```bash
cp deploy/docker/.env.example deploy/docker/.env
```

Defaults:

```dotenv
MYSQL_HOST=host.docker.internal
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=root
MYSQL_DATABASE=dtm
DTM_ENDPOINT=grpc://host.docker.internal:36790
```

On Linux, `host.docker.internal` is provided by the compose `extra_hosts`
mapping. MySQL must listen on an address reachable through the Docker host
gateway; binding only `127.0.0.1` may not be reachable from a Linux container.

If MySQL is another Docker container, attach it to the `mall-sim` network and
use its container name:

```bash
docker network connect mall-sim mysql-unicorn
# .env
MYSQL_HOST=mysql-unicorn
```

## 3. Start etcd and DTM

```bash
docker compose -f deploy/docker/docker-compose-sim.yaml up -d etcd dtm
docker compose -f deploy/docker/docker-compose-sim.yaml ps
```

Checks:

```bash
curl http://127.0.0.1:36789/api/dtmsvr/version
docker compose -f deploy/docker/docker-compose-sim.yaml exec etcd \
  etcdctl --endpoints=http://127.0.0.1:2379 endpoint health
```

## 4. Host service environment

For every service that participates in a DTM branch, register an endpoint that
the DTM container can reach:

```bash
export ETCD_ENDPOINTS=127.0.0.1:2379
export DTM_ADDRESS=127.0.0.1:36790

# inventory, order, pricing, payment ...
export GRPC_ENDPOINT=grpc://host.docker.internal:9003
```

Without `GRPC_ENDPOINT`, Kratos registers the container-local/listen address or
`inventory:9003`; DTM cannot resolve that from its own container.

On Linux, `host.docker.internal` is mapped by the compose `extra_hosts` entry.
If you use another host address, set the same address in `GRPC_ENDPOINT` and
`DTM_ENDPOINT`.

## 5. Other dependencies

etcd + DTM alone are not enough for the whole mall:

| Dependency | Needed by |
|---|---|
| MySQL | every service, plus DTM store |
| Redis | catalog (and future cart cache) |
| NATS JetStream | outbox/event consumers |

Use `deploy/compose/infra.yml` for MySQL/Redis/NATS, or provide host instances
and point `MYSQL_DSN`, `REDIS_ADDR`, `NATS_URL` at them.

## Troubleshooting

- `dtm` exits with a MySQL connection error: `STORE_HOST` is not reachable.
  Test from the container:
  `docker compose -f deploy/docker/docker-compose-sim.yaml run --rm dtm sh`.
- DTM finds the service but the branch call times out: the service registered
  `grpc://inventory:9003` instead of a host-reachable endpoint. Restart it with
  `GRPC_ENDPOINT=grpc://host.docker.internal:9003`.
- etcd data is stale after changing the cluster: remove `deploy/docker/data/etcd`
  (this deletes local etcd state) or use a fresh named volume.

## etcd image and data volume

`quay.io/coreos/etcd:v3.5.17` is the legacy CoreOS namespace for the official
etcd image. CoreOS created etcd, but the project is now CNCF-governed; the
namespace is historical, not a special etcd distribution. For new deployments,
pin an exact version (or digest) from the official etcd image and verify the
platform/architecture you need. Do not use `latest` for infrastructure.

The compose file uses a named volume:

```yaml
volumes:
  - etcd-data:/etcd-data
```

`mall-sim-etcd-data` is managed by Docker. It survives `docker compose down`
but is removed by `docker compose down -v`. A bind mount such as
`./data/etcd:/etcd-data` is convenient for local inspection but is easier to
delete accidentally and can hit host permission issues. Use one etcd data
directory per etcd member; never share it between nodes.
