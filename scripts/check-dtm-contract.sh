#!/usr/bin/env bash
# Static contract guard for the DTM wiring. This is not a substitute for the
# Docker smoke test; it prevents accidental removal of the required pieces.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
fail=0

need() {
  local file="$1" pattern="$2" label="$3"
  if ! grep -q "$pattern" "$file"; then
    echo "[dtm-contract] missing $label in $file"
    fail=1
  fi
}

need internal/services/order/service.go 'dtm.New\|s\.dtm\.Saga' 'order creates a DTM saga'
need internal/services/order/service.go 'CreateOrderInner' 'order create branch'
need internal/services/order/service.go 'CancelOrderInner' 'order cancel branch'
need internal/services/order/service.go 'INVENTORY_DTM_URL' 'inventory branch URL'
need internal/services/inventory/service.go 'dtm.Branch' 'inventory barrier'
need internal/services/inventory/service.go 'Reserve\|Confirm\|Release' 'inventory branch operations'
need internal/services/pricing/service.go 'dtm.Branch' 'pricing barrier'
need internal/services/payment/tcc.go 'TccGlobalTransaction\|TCC(' 'payment TCC orchestration'
need internal/services/order/tcc.go 'TccConfirmMarkPaid' 'order TCC branch'
need internal/services/inventory/tcc.go 'TccConfirm' 'inventory TCC branch'
need internal/services/pricing/tcc.go 'TccConsume\|TccConfirmConsume' 'pricing TCC branch'
need deploy/compose/dtm/config.yml 'dtm-driver-kratos' 'DTM kratos discovery driver'
need deploy/compose/infra.yml 'dtmphp/dtm' 'DTM image'

if grep -R --include='*.go' -n 'MustGenGid\|MustGetDtmClient' internal cmd >/dev/null 2>&1; then
  echo "[dtm-contract] nondeterministic DTM GID helper found"
  fail=1
fi

if [ "$fail" -ne 0 ]; then exit 1; fi
echo "[dtm-contract] OK (static; run make dtm-smoke with the stack up)"
