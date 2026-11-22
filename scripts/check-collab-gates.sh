#!/usr/bin/env bash
# ADR 0006 entrypoint. warn never blocks; strict blocks on any failure.
set -euo pipefail
cd "$(dirname "$0")/.."
mode="${GATES_MODE:-warn}"
args=()
for arg in "$@"; do
  case "$arg" in
    --mode=warn|--mode=strict) mode="${arg#--mode=}" ;;
    *) args+=("$arg") ;;
  esac
done
if [ ${#args[@]} -gt 0 ]; then
  exec python3 scripts/collab_gates.py --mode="$mode" "${args[@]}"
fi
exec python3 scripts/collab_gates.py --mode="$mode"
