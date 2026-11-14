#!/usr/bin/env bash
# Guard: outbox migrations must only live in databases that define outbox_events.
set -euo pipefail
cd "$(dirname "$0")/.."
fail=0
for dir in migrations/*/; do
  service="$(basename "$dir")"
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
