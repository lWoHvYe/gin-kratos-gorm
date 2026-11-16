#!/usr/bin/env bash
# Recover the dev k3s Ambient data plane after workload certificates expire.
# Idempotent: restarts ztunnel/istiod, recreates the waypoint pod, restarts the
# ingress gateway and the mall deployments, then waits for PROGRAMMED=True.
set -euo pipefail
cd "$(dirname "$0")/.."

echo ">> restarting ztunnel and istiod"
kubectl -n istio-system rollout restart ds/ztunnel
kubectl -n istio-system rollout restart deploy/istiod
kubectl -n istio-system rollout status ds/ztunnel --timeout=240s
kubectl -n istio-system rollout status deploy/istiod --timeout=240s

echo ">> restarting the waypoint and ingress gateway"
kubectl -n mall delete pod -l gateway.networking.k8s.io/gateway-name=mall-waypoint --ignore-not-found
kubectl -n mall scale deployment mall-waypoint --replicas=1
kubectl -n istio-system rollout restart deployment/mall-gateway-istio

echo ">> restarting mall workloads"
kubectl -n mall rollout restart deployment
kubectl -n mall rollout status deployment --timeout=300s

for _ in $(seq 1 40); do
  programmed=$(kubectl -n mall get gateway mall-waypoint -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null || true)
  [ "$programmed" = "True" ] && break
  sleep 3
done
if [ "${programmed:-}" != "True" ]; then
  echo "ambient-recover: waypoint not Programmed; check istiod/waypoint" >&2
  exit 1
fi
echo "ambient-recover: OK"
