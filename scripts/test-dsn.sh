#!/bin/sh
# Derive a per-package integration database from the base DSN.
# Usage: scripts/test-dsn.sh "$TEST_MYSQL_DSN" inventory
set -eu

dsn="${1:?dsn required}"
suffix="${2:?suffix required}"
prefix="${TEST_DB_PREFIX:-mall_integration}"

case "$dsn" in
  *"/mall_integration_test?"*)
    printf '%s' "$dsn" | sed "s#/mall_integration_test?#/${prefix}_${suffix}?#"
    ;;
  *"/mall_integration_test")
    printf '%s' "$dsn" | sed "s#/mall_integration_test\$#/${prefix}_${suffix}#"
    ;;
  *)
    echo "TEST_MYSQL_DSN must contain /mall_integration_test (with optional ?params)" >&2
    exit 1
    ;;
esac
