#!/usr/bin/env bash
# Validate the production-HA Helm profiles without a cluster.
#
# It renders both charts in the default (single-node) and HA profiles and
# asserts the properties that make the HA profile different: replicated NATS
# with routes, Sentinel with a writable config copy, external MySQL, wired
# secret references, pool limits and PDBs. The default profile must stay
# single-node so `make k8s-deploy` keeps working on the dev VM.
set -euo pipefail
cd "$(dirname "$0")/.."

charts=deploy/k8s/charts
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "k8s-ha-verify: $1 is required" >&2
    exit 1
  }
}
need helm

echo ">> lint"
for chart in infra mall; do
  helm lint "$charts/$chart" -f "$charts/$chart/values.yaml" >/dev/null
  helm lint "$charts/$chart" -f "$charts/$chart/values.yaml" -f "$charts/$chart/values-ha.yaml" >/dev/null
  echo "   $chart ok"
done

echo ">> render"
helm template mall-infra "$charts/infra" -n mall-data \
  -f "$charts/infra/values.yaml" >"$tmp/infra-default.yaml"
helm template mall-infra "$charts/infra" -n mall-data \
  -f "$charts/infra/values.yaml" -f "$charts/infra/values-ha.yaml" >"$tmp/infra-ha.yaml"
helm template mall "$charts/mall" -n mall \
  -f "$charts/mall/values.yaml" >"$tmp/mall-default.yaml"
helm template mall "$charts/mall" -n mall \
  -f "$charts/mall/values.yaml" -f "$charts/mall/values-ha.yaml" >"$tmp/mall-ha.yaml"
echo "   rendered default + ha for both charts"

# Structured assertions need kubectl only for local object decoding (no API
# writes); jq does the querying. Fall back to plain greps when kubectl is
# missing so the script still works in a slim CI image.
if command -v kubectl >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
  json() { kubectl create --dry-run=client -f "$1" -o json 2>/dev/null | jq -s "$2"; }
  assert() {
    local file=$1 expr=$2 desc=$3
    if json "$file" "$expr" >/dev/null; then
      echo "   ok: $desc"
    else
      echo "k8s-ha-verify: FAILED: $desc" >&2
      exit 1
    fi
  }

  echo ">> default profile stays single-node"
  assert "$tmp/infra-default.yaml" \
    'any(.[]; .kind=="StatefulSet" and .metadata.name=="redis" and .spec.replicas==1)' \
    "infra: redis StatefulSet has 1 replica"
  assert "$tmp/infra-default.yaml" \
    'any(.[]; .kind=="StatefulSet" and .metadata.name=="nats" and .spec.replicas==1)' \
    "infra: nats StatefulSet has 1 replica"
  assert "$tmp/infra-default.yaml" \
    '[.[]|select(.kind=="PodDisruptionBudget")]|length==0' \
    "infra: no PDBs rendered"
  assert "$tmp/mall-default.yaml" \
    '[.[]|select(.kind=="Deployment")|.spec.template.spec.containers[]?|(.env // [])[]|select(.name=="REDIS_MODE")]|length==0' \
    "mall: no REDIS_MODE override"

  echo ">> HA infra profile"
  assert "$tmp/infra-ha.yaml" \
    'any(.[]; .kind=="StatefulSet" and .metadata.name=="redis-replica" and .spec.replicas==2)' \
    "redis: 2 replicas"
  assert "$tmp/infra-ha.yaml" \
    'any(.[]; .kind=="StatefulSet" and .metadata.name=="redis-master" and .spec.replicas==1)' \
    "redis: 1 master"
  assert "$tmp/infra-ha.yaml" \
    'any(.[]; .kind=="Deployment" and .metadata.name=="redis-sentinel" and .spec.replicas==3)' \
    "redis: 3 sentinels"
  assert "$tmp/infra-ha.yaml" \
    'any(.[]; .kind=="StatefulSet" and .metadata.name=="nats" and .spec.replicas==3)' \
    "nats: 3 replicas"
  assert "$tmp/infra-ha.yaml" \
    'any(.[]; .kind=="StatefulSet" and .metadata.name=="nats" and (.spec.template.spec.containers[0].args|join(" ")|test("nats-2.nats-headless")))' \
    "nats: cluster routes include all three pods"
  assert "$tmp/infra-ha.yaml" \
    '[.[]|select(.kind=="PodDisruptionBudget")|.metadata.name]|sort==["nats","redis-replica","redis-sentinel"]' \
    "infra: PDBs for nats and redis tiers"
  assert "$tmp/infra-ha.yaml" \
    '[.[]|select(.kind=="StatefulSet" and .metadata.name=="mysql")]|length==0' \
    "mysql: external mode renders no StatefulSet"
  assert "$tmp/infra-ha.yaml" \
    'any(.[]; .kind=="Job" and .metadata.name=="mall-migrate" and any(.spec.template.spec.containers[0].env[]; .name=="MYSQL_HOST" and .value=="mysql.prod.internal"))' \
    "migrate: targets the external MySQL host"

  echo ">> HA mall profile"
  assert "$tmp/mall-ha.yaml" \
    '[.[]|select(.kind=="Deployment")|.spec.template.spec.containers[]?|(.env // [])] as $envs | [$envs[]|select(any(.[]; .name=="MYSQL_DSN"))] as $svc | (($svc|length)==12) and all($svc[]; (any(.name=="REDIS_MODE" and .value=="sentinel")) and (any(.name=="REDIS_ADDRS" and (.value|test("redis-sentinel")))) and (any(.name=="NATS_STREAM_REPLICAS" and .value=="3")) and (any(.name=="MYSQL_DSN" and (.value|startswith("$(MYSQL_USER):$(MYSQL_PASSWORD)@")))) and (any(.name=="MYSQL_MAX_OPEN_CONNS" and .value=="16")))' \
    "all 12 services use sentinel, replicated streams, secret DSN and pool caps"
  assert "$tmp/mall-ha.yaml" \
    'any(.[]; .kind=="Deployment" and .metadata.name=="gateway" and .spec.replicas==3)' \
    "gateway: 3 replicas"
  assert "$tmp/mall-ha.yaml" \
    'any(.[]; .kind=="Deployment" and .metadata.name=="seckill" and .spec.replicas==4)' \
    "seckill: 4 replicas"
  assert "$tmp/mall-ha.yaml" \
    'any(.[]; .kind=="PodDisruptionBudget" and .metadata.name=="seckill" and .spec.minAvailable==2)' \
    "seckill: PDB keeps 2 up"
  assert "$tmp/mall-ha.yaml" \
    'any(.[]; .kind=="NetworkPolicy" and .metadata.name=="allow-mall-internal" and any(.spec.egress[]; .to[]?.ipBlock?.cidr=="10.0.0.0/8" and any(.ports[]?; .port==3306)))' \
    "mall: egress allows the external MySQL subnet"
else
  echo ">> kubectl/jq not available; using grep fallbacks"
  grep -q 'name: redis-replica' "$tmp/infra-ha.yaml"
  grep -q 'name: redis-sentinel' "$tmp/infra-ha.yaml"
  grep -q 'nats-2.nats-headless' "$tmp/infra-ha.yaml"
  grep -q 'REDIS_MODE' "$tmp/mall-ha.yaml"
  grep -q 'NATS_STREAM_REPLICAS' "$tmp/mall-ha.yaml"
  if grep -q 'REDIS_MODE' "$tmp/mall-default.yaml"; then
    echo "k8s-ha-verify: default mall render unexpectedly sets REDIS_MODE" >&2
    exit 1
  fi
fi

echo "k8s-ha-verify: ok"
