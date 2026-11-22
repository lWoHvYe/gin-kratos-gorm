#!/bin/sh
# Contract gate for the cross-service surfaces that carry group/refund facts.
set -eu
cd "$(dirname "$0")/.."
python3 scripts/check-proto-evolution.py
go test ./internal/platform/events -count=1
go test ./internal/services/admin ./internal/services/gateway -run 'TestOpenAPI' -count=1
