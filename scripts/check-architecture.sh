#!/usr/bin/env bash
# Lightweight architecture guard for the microservice mall.
# These checks are intentionally dependency-free and run in `make verify`.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
fail=0

# 1. No service may import another service package. Cross-service calls must go
#    through generated gRPC clients or platform/contracts.
cross="$(go list -f '{{.ImportPath}} {{join .Imports "\n"}}' ./internal/services/... 2>/dev/null | awk '
  NF {
    pkg=$1; if (substr(pkg,1,33)!="gin-kratos-gorm/internal/services/") next;
    svc=pkg; sub("gin-kratos-gorm/internal/services/","",svc); sub("/.*","",svc);
    for (i=2;i<=NF;i++) {
      imp=$i;
      if (substr(imp,1,33)=="gin-kratos-gorm/internal/services/") {
        other=imp; sub("gin-kratos-gorm/internal/services/","",other); sub("/.*","",other);
        if (other!=svc) print pkg " imports " imp;
      }
    }
  }')"
if [ -n "$cross" ]; then
  echo "[arch] cross-service imports found:"
  echo "$cross"
  fail=1
fi

# 2. Platform must not depend on service code.
if grep -R --include='*.go' -n 'gin-kratos-gorm/internal/services/' internal/platform >/tmp/mall-platform.$$ 2>/dev/null; then
  echo "[arch] internal/platform imports a service:"
  cat /tmp/mall-platform.$$
  fail=1
fi
rm -f /tmp/mall-platform.$$

# 3. GORM is allowed only in the catalog/cart read-side repositories. The
# transactional core must stay on database/sql.
gorm_hits="$(grep -R --include='*.go' -n 'gorm.io\|glebarez/sqlite' internal cmd 2>/dev/null | grep -v -E '^internal/services/(catalog|cart)/gorm_repo.go:' || true)"
if [ -n "$gorm_hits" ]; then
  echo "[arch] GORM/sqlite import outside catalog read side:"
  echo "$gorm_hits"
  fail=1
fi

# 4. DTM GIDs must be deterministic; never call MustGenGid/MustGetDtmClient.
if grep -R --include='*.go' -n 'MustGenGid\|MustGetDtmClient' internal cmd >/dev/null 2>&1; then
  echo "[arch] nondeterministic DTM GID helper found"
  grep -R --include='*.go' -n 'MustGenGid\|MustGetDtmClient' internal cmd || true
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "[arch] OK"
