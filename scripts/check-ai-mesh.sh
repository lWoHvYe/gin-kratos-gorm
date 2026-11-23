#!/usr/bin/env bash
# Live #38 check: AI service account can read Catalog but cannot call a write
# method through the ambient waypoint. UAT-only by default.
set -euo pipefail
NS="${UAT_NS:-mall-uat}"
IMAGE="${AI_MESH_IMAGE:-fullstorydev/grpcurl:v1.9.3}"
case "$NS" in mall-uat*) ;; *) echo "refusing outside mall-uat" >&2; exit 1;; esac
cleanup() { kubectl -n "$NS" delete pod ai-mesh-read ai-mesh-write ai-mesh-pricing --ignore-not-found >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
kubectl -n "$NS" delete pod ai-mesh-read ai-mesh-write ai-mesh-pricing --ignore-not-found >/dev/null

kubectl -n "$NS" run ai-mesh-read --image="$IMAGE" --restart=Never \
  --overrides='{"spec":{"serviceAccountName":"ai-sa"}}' --command -- \
  grpcurl -plaintext -d '{}' catalog:9002 mall.v1.CatalogService/ListProducts >/dev/null
for _ in $(seq 1 30); do
  [ "$(kubectl -n "$NS" get pod ai-mesh-read -o jsonpath='{.status.phase}' 2>/dev/null)" = "Succeeded" ] && break
  sleep 1
done
READ="$(kubectl -n "$NS" logs ai-mesh-read 2>&1 || true)"
echo "$READ" | grep -q '"products"' || { echo "AI read failed: $READ"; exit 1; }
echo "AI read allowed"

kubectl -n "$NS" run ai-mesh-write --image="$IMAGE" --restart=Never \
  --overrides='{"spec":{"serviceAccountName":"ai-sa"}}' --command -- \
  grpcurl -plaintext -d '{}' catalog:9002 mall.v1.CatalogService/CreateProduct >/dev/null
for _ in $(seq 1 30); do
  [ "$(kubectl -n "$NS" get pod ai-mesh-write -o jsonpath='{.status.phase}' 2>/dev/null)" = "Succeeded" ] && break
  sleep 1
done
WRITE="$(kubectl -n "$NS" logs ai-mesh-write 2>&1 || true)"
echo "$WRITE" | grep -q 'PermissionDenied' || { echo "AI write was not denied: $WRITE"; exit 1; }
echo "AI write denied (PermissionDenied)"

# 2026-11-23 (H5): after narrowing allow-ingress, the AI SA must also lose any
# PricingService write surface (DENY stopgap + no matching ALLOW).
kubectl -n "$NS" run ai-mesh-pricing --image="$IMAGE" --restart=Never \
  --overrides='{"spec":{"serviceAccountName":"ai-sa"}}' --command -- \
  grpcurl -plaintext -d '{}' pricing:9005 mall.v1.PricingService/LockCoupon >/dev/null
for _ in $(seq 1 30); do
  [ "$(kubectl -n "$NS" get pod ai-mesh-pricing -o jsonpath='{.status.phase}' 2>/dev/null)" = "Succeeded" ] && break
  sleep 1
done
PRICING="$(kubectl -n "$NS" logs ai-mesh-pricing 2>&1 || true)"
echo "$PRICING" | grep -q 'PermissionDenied' || { echo "AI pricing write was not denied: $PRICING"; exit 1; }
echo "AI pricing write denied (PermissionDenied)"
