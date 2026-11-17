#!/usr/bin/env bash
# Guard migration naming/placement:
# - every file uses a four-digit numeric prefix;
# - within one service, prefixes >= 0008 are unique (<=0007 legacy duplicates
#   stay as-is: the ledger keys by '<service>/<filename>', so renumbering would
#   replay an applied migration);
# - outbox migrations only live where 0001_init.sql creates outbox_events.
set -euo pipefail
cd "$(dirname "$0")/.."
fail=0
for dir in migrations/*/; do
  service="$(basename "$dir")"

  for file in "${dir}"*.sql; do
    base="$(basename "$file")"
    case "$base" in
      [0-9][0-9][0-9][0-9]_*.sql) ;;
      *)
        echo "[migrations] ${service}/${base} does not start with a four-digit prefix" >&2
        fail=1
        ;;
    esac
  done

  duplicates="$(find "$dir" -maxdepth 1 -type f -name '*.sql' -exec basename {} \; \
    | cut -c1-4 | sort | uniq -d)"
  for prefix in $duplicates; do
    if [ $((10#$prefix)) -ge 8 ]; then
      echo "[migrations] ${service} has duplicate prefix ${prefix} (>= 0008)" >&2
      fail=1
    fi
  done

  has_table=0
  grep -q "outbox_events" "${dir}0001_init.sql" 2>/dev/null && has_table=1
  has_migrations=0
  ls "${dir}"*outbox*.sql >/dev/null 2>&1 && has_migrations=1
  if [ "$has_migrations" -eq 1 ] && [ "$has_table" -eq 0 ]; then
    echo "[migrations] ${service} has outbox migrations but no outbox_events table" >&2
    fail=1
  fi
done
if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "[migrations] OK"
