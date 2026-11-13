#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== repository"
echo "path: $(pwd)"
echo "branch: $(git branch --show-current)"
echo "last: $(git log -1 --oneline)"
echo "status:"
git status --short --untracked-files=all | head -30 || true

echo
echo "== key docs"
printf '%s\n' docs/handoff.md docs/access.md docs/seckill-design.md docs/services/seckill.md docs/ops.md

echo
echo "== live cluster"
if command -v kubectl >/dev/null 2>&1; then
  kubectl -n mall get pods --no-headers 2>/dev/null | head -25 || true
  echo "-- gateway"
  kubectl -n istio-system get svc mall-gateway-istio 2>/dev/null || true
else
  echo "kubectl not available"
fi

echo
echo "== access"
./scripts/show-access.sh 2>/dev/null || true

echo
echo "== next"
echo "read docs/handoff.md; run make verify / make seckill-integration when the stack is up"
