#!/usr/bin/env bash
# Apply the explicit demo catalog (fictional brands) to a dev/staging stack.
# Usage: scripts/seed-demo-catalog.sh [--k8s|--compose]
set -euo pipefail
cd "$(dirname "$0")/.."
mode="${1:---k8s}"
sql="deploy/seed/catalog-demo.sql"
case "$mode" in
  --k8s)
    kubectl -n mall-data exec -i mysql-0 -- mysql --default-character-set=utf8mb4 -uroot -proot < "$sql"
    # Product detail is cached in Redis; drop the demo entries after a reseed.
    kubectl -n mall-data exec redis-0 -- sh -c \
      'for k in $(redis-cli --scan --pattern "catalog:product:*"); do redis-cli DEL "$k" >/dev/null; done'
    ;;
  --compose)
    docker compose -f deploy/compose/infra.yml exec -T mysql mysql --default-character-set=utf8mb4 -uroot -proot < "$sql"
    docker compose -f deploy/compose/infra.yml exec -T redis sh -c \
      'for k in $(redis-cli --scan --pattern "catalog:product:*"); do redis-cli DEL "$k" >/dev/null; done'
    ;;
  *)
    echo "usage: $0 [--k8s|--compose]" >&2
    exit 1
    ;;
esac
echo "seed-demo-catalog: applied $sql ($mode)"
