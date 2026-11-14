# Production HA profile

This is the runbook for the opt-in HA overlay of the two Helm charts. It follows
the design in [seckill-design.md](seckill-design.md) section 19 (1M users, 10
SKUs, 100 units each) and the handoff priority "production HA manifests/runbook".

The default profile stays the single-node k3s dev stack. Nothing here is enabled
until you pass `-f .../values-ha.yaml`, so `make k8s-deploy` keeps working.

## 1. What this profile does and does not do

Rendered by the charts:

- Redis Sentinel reference topology: 1 master, 2 replicas, 3 sentinels, with
  optional AUTH from a Secret and PDBs.
- NATS: 3-node JetStream cluster with routes, `nats-headless` service, PDB, and
  `NATS_STREAM_REPLICAS=3` for durable streams created after install.
- MySQL: external mode (managed service or operator) with the migration Job
  targeting the write endpoint; `max_connections` and a pool cap per service.
- Mall: 2-4 replicas per service, PDBs, Sentinel/stream env wiring, and MySQL
  credentials referenced from a Secret instead of inlined in values.

Deliberately not rendered here:

- MySQL replication/Group Replication/InnoDB Cluster. Use a managed MySQL or a
  MySQL operator (Percona XtraDB Cluster, MySQL Operator, cloud RDS/Aurora).
  Hand-rolling primary failover in a Helm chart is how data gets corrupted.
- Redis Cluster slot management. The client supports cluster mode
  (`REDIS_MODE=cluster`) and seckill keys already use `{activity_id}` hash tags,
  but slot creation/resharding should come from an operator or a managed Redis.
- Edge WAF/Waiting Room. That is the next handoff item; this profile only makes
  the origin able to survive a shaped burst.
- A 1M-user load test. Do not run one against the dev VM.

## 2. Topology

```text
edge WAF / waiting room        (next item; caps origin RPS)
        │
Istio ingress gateway          (3+ replicas, prescaled)
        │
mall gateway                   (3 replicas, PDB minAvailable 2)
        │
seckill-api                    (4 replicas, PDB minAvailable 2)
  ├─ Redis Sentinel 1M/2R/3S   (dedicated, AUTH on, PDBs)
  ├─ NATS JetStream 3 nodes    (stream replicas=3, PDB minAvailable 2)
  └─ MySQL                     (external HA write endpoint, migrations via Job)
```

Sizing rule of thumb for the target envelope (100k RPS at the origin, ≤1k
successful orders): scale the admission tier (gateway + seckill-api) and keep
Redis off the catalog cache. The database only sees ~1k writes; do not scale
MySQL for the 999k losers.

## 3. Prerequisites

- A multi-node cluster with a default StorageClass that supports
  `ReadWriteOnce` and, ideally, zone-aware volumes.
- Namespaces from `deploy/k8s/manifests/namespaces.yaml` (`mall`, `mall-data`).
- Images pushed to a registry reachable by the cluster (`make k8s-images`,
  `make k8s-import` for the dev VM).
- An external MySQL write endpoint with all `mall_*` databases and a user that
  can run migrations.
- Two Secrets. Secret references are namespace-local, so the same Redis password
  must exist in both namespaces; keep one private values file as the source.

```bash
# Values are never committed. Create the MySQL credential in the mall namespace.
kubectl -n mall create secret generic mall-mysql-credentials \
  --from-literal=password='<db-password>'

# Redis AUTH: the infra chart renders mall-data/redis-credentials from
# redis.password; create the matching Secret in the mall namespace.
kubectl -n mall create secret generic redis-credentials \
  --from-literal=password='<redis-password>'

# Application secrets. Production mode refuses to start with the dev JWT
# default, and the payment service refuses to start without a callback secret
# because mock payments are off in this profile.
kubectl -n mall create secret generic mall-secrets \
  --from-literal=jwt-secret="$(openssl rand -hex 32)" \
  --from-literal=seckill-token-secret="$(openssl rand -hex 32)" \
  --from-literal=payment-secret="$(openssl rand -hex 32)"
```

The HA overlay sets `appEnv: production`, disables the mock pay route
(`payment.mockEnabled: "false"`) and reads all three keys from `mall-secrets`.
`/secure/mall-ha-secrets.yaml` only carries the Redis password (not in git):

```yaml
redis:
  password: "<redis-password>"
```

The MySQL password must be DSN-safe (URL-encoded if it contains `@`, `:`, `/`
or `#`) because it is expanded into the DSN via `$(MYSQL_PASSWORD)`.

## 4. Install

```bash
helm upgrade --install mall-infra deploy/k8s/charts/infra -n mall-data \
  -f deploy/k8s/charts/infra/values.yaml \
  -f deploy/k8s/charts/infra/values-ha.yaml \
  -f /secure/mall-ha-secrets.yaml --wait --timeout 15m

helm upgrade --install mall deploy/k8s/charts/mall -n mall \
  -f deploy/k8s/charts/mall/values.yaml \
  -f deploy/k8s/charts/mall/values-ha.yaml \
  --wait --timeout 15m
```

Change `mysql.prod.internal` in both overlays to the real write endpoint before
installing. The migration Job runs as a Helm hook and is safe to rerun.

The mall namespace denies egress by default, so set `network.externalMySQLCIDR`
to the database subnet (the overlay ships `10.0.0.0/8` as a placeholder). Without
it, pods resolve DNS but the MySQL connection is dropped.

Validate the rendered profile without a cluster (this is also what CI can run):

```bash
make k8s-ha-verify     # lint + render + topology assertions
make k8s-render-ha     # writes /tmp/{mall,mall-infra}-ha-rendered.yaml
```

## 5. Connection limits

Each service pod opens its own pool. The hard rule:

```text
sum(service replicas × mysql.maxOpenConns) + headroom < MySQL max_connections
```

With the shipped overlay: 12 services, average 2.3 replicas, 16 open
connections each ≈ 440 connections; set `max_connections` ≥ 500 (the infra
overlay's standalone reference uses 500). For the read-heavy seckill burst,
budget Redis connections separately; the app uses one pool per pod.

Raise pool sizes only after measuring wait time (`sql.DBStats.WaitCount`), and
raise `max_connections` first.

## 6. Redis failover

The Sentinel profile resolves the configured master hostname
(`resolve-hostnames yes`, Redis 7+; the chart pins 8.x) but keeps announcing pod
IPs, because the mall namespace must be able to dial the address Sentinel
returns. Sentinels run from a writable copy of their config because failover
rewrites it, and the config is mounted read-only.

Drill (staging, not during a sale):

```bash
# Force a failover and watch the app keep admitting.
kubectl -n mall-data exec -it redis-sentinel-<pod> -- \
  redis-cli -p 26379 sentinel failover mallmaster

# Confirm the new master and that replicas re-pointed.
kubectl -n mall-data exec -it redis-sentinel-<pod> -- \
  redis-cli -p 26379 sentinel get-master-addr-by-name mallmaster

# The app must reconnect on its own; no restart should be needed.
kubectl -n mall logs deploy/seckill --tail=50
```

Expectations and RPO: replication is asynchronous, so a failover can lose the
last few writes. Seckill tolerates this because the DB hold/order path is the
source of truth and the reconciler reports Redis/DB drift; normal catalog cache
entries simply miss. A failover during a sale is still a visible incident —
freeze deploys and run the drill before the sale.

Known caveat: because the announced address is a pod IP, a pod that is recreated
with a new IP is only re-adopted through failover; a returning old master can
briefly look like an empty master and is reconfigured as a replica. It receives
no client traffic (clients use Sentinels), but this is exactly why a managed
Redis or a Redis operator with stable identities is the recommended production
option.

## 7. NATS streams

`NATS_STREAM_REPLICAS=3` only affects streams the app creates from scratch
(`MALL` and the seckill work-queue stream). Streams created by the single-node
profile keep `replicas=1`; migrate them explicitly:

The `nats` server image does not ship the CLI, so use a throwaway nats-box pod:

```bash
kubectl -n mall-data run nats-box --rm -it --restart=Never \
  --image=natsio/nats-box -- \
  nats -s nats://nats:4222 stream edit MALL --replicas 3
kubectl -n mall-data run nats-box --rm -it --restart=Never \
  --image=natsio/nats-box -- \
  nats -s nats://nats:4222 stream edit SECKILL --replicas 3

kubectl -n mall-data run nats-box --rm -it --restart=Never \
  --image=natsio/nats-box -- nats -s nats://nats:4222 server list
```

Do this in a maintenance window: changing replicas re-creates the stream
internals. Verify with `nats stream info MALL` (`Replicas: 3`) and that
`nats server list` shows three nodes.

## 8. Pre-sale checklist

1. `make k8s-ha-verify` and `helm lint` pass for the exact values you deploy.
2. MySQL: write endpoint healthy, `max_connections` sized, backups verified,
   migration Job completed.
3. Redis: `sentinel get-master-addr-by-name` matches the expected master, AUTH
   works, all three sentinels see each other (`sentinel sentinels mallmaster`).
4. NATS: three nodes in `nats server list`, `MALL` stream replicas=3, DLQ
   subject has a consumer and an owner.
5. Pre-scale gateway and seckill-api to the sale size; HPA stays a safety net
   (it is off in the overlay on purpose; HPA/PDB validation is a separate item).
6. Dashboards show admission RPS, `mall_seckill_admissions_total`,
   `mall_seckill_order_commands_total{result="dlq"}`, Redis p99/CPU, 429/503.
7. Failover drills completed this week; no pending rollouts during the sale.
8. Demo accounts removed/rotated; ingress TLS and the real hostname in place.

## 9. Rolling upgrades

PDBs keep `gateway` and `seckill` at 2 available. Draining a node evicts at most
one pod per workload; Redis master loss triggers Sentinel failover, NATS loses
one replica and keeps quorum (3 nodes). Never drain two NATS pods at once.

The migration Job runs pre-upgrade, so the schema moves before new pods roll.
Keep migrations backward compatible for one release.

## 10. What is still open

- Edge admission/WAF and origin RPS caps (handoff item 2).
- HPA/PDB validation under load (item 3).
- Bulk-create ten activities under one campaign (item 4).
- DLQ/admin operations UI (item 5).
- Distributed k6 in an isolated performance environment (item 6).

Do not treat this profile as a 1M-RPS claim. It makes the origin replicable,
failover-tested in staging, and bounded; the edge still has to shape the crowd.
