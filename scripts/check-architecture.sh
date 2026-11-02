#!/usr/bin/env bash
# Lightweight architecture guard for the microservice mall.
# These checks are intentionally dependency-free and run in `make verify`.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
fail=0

# 1. No service may import another service package. Cross-service calls must go
#    through generated gRPC clients or platform/contracts.
if grep -R --include='*.go' -n 'gin-kratos-gorm/internal/services/' internal/services cmd \
  | grep -v '^internal/services/[^:]*:' \
  | grep -v '^cmd/' >/tmp/mall-cross-service.$$ 2>/dev/null; then
  echo "[arch] cross-service imports found:"
  cat /tmp/mall-cross-service.$$
  fail=1
fi
rm -f /tmp/mall-cross-service.$$

# 2. Platform must not depend on service code.
if grep -R --include='*.go' -n 'gin-kratos-gorm/internal/services/' internal/platform >/tmp/mall-platform.$$ 2>/dev/null; then
  echo "[arch] internal/platform imports a service:"
  cat /tmp/mall-platform.$$
  fail=1
fi
rm -f /tmp/mall-platform.$$

# 3. GORM is allowed only in the catalog read-side repository. The
# transactional core must stay on database/sql.
gorm_hits="$(grep -R --include='*.go' -n 'gorm.io\|glebarez/sqlite' internal cmd 2>/dev/null | grep -v '^internal/services/catalog/gorm_repo.go:' || true)"
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
