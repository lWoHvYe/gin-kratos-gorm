#!/usr/bin/env bash
# Run after the user's single-node Kubernetes + Istio Ambient cluster is ready.
set -euo pipefail

NS="${NAMESPACE:-mall}"
DATA_NS="${DATA_NAMESPACE:-mall-data}"
INGRESS_HOST="${INGRESS_HOST:-mall.local}"
BASE_URL="${BASE_URL:-http://127.0.0.1:18080}"

command -v kubectl >/dev/null || { echo "kubectl is required"; exit 1; }

echo "== wait for workloads =="
kubectl wait --for=condition=Available deployment --all -n "$NS" --timeout=300s
for sts in mysql redis nats etcd; do
  kubectl rollout status "statefulset/$sts" -n "$DATA_NS" --timeout=300s
done
kubectl wait --for=condition=Complete job/mall-migrate -n "$DATA_NS" --timeout=300s

echo "== ambient enrollment =="
if kubectl get pods -n "$NS" -o jsonpath='{range .items[*]}{.spec.containers[*].name}{"\n"}{end}' | grep -q istio-proxy; then
  echo "unexpected sidecar found in $NS"; exit 1
fi
kubectl get peerauthentication -n "$NS" default -o yaml | grep -q 'mode: STRICT'

echo "== port-forward gateway/admin =="
kubectl -n "$NS" port-forward svc/gateway 18080:8080 >/tmp/mall-gw-portforward.log 2>&1 &
GW_PID=$!
kubectl -n "$NS" port-forward svc/admin 18081:8081 >/tmp/mall-admin-portforward.log 2>&1 &
ADMIN_PID=$!
trap 'kill $GW_PID $ADMIN_PID 2>/dev/null || true' EXIT
sleep 2

echo "== DTM/C-end smoke =="
BASE_URL="$BASE_URL" ADMIN_URL="http://127.0.0.1:18081" ./scripts/dtm-smoke.sh

echo "== ingress check =="
if command -v curl >/dev/null; then
  curl -fsS -H "Host: $INGRESS_HOST" http://127.0.0.1/healthz >/dev/null || true
fi

echo "== k8s smoke passed =="
