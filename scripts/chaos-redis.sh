#!/usr/bin/env bash
# Redis outage chaos for seckill admission.
#
# Starts an isolated Redis, runs the admission test in the background, stops
# Redis mid-flight, restarts it, and requires admission to fail closed and then
# recover without consuming extra stock.
set -euo pipefail
cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-docker}"
if ! command -v "$DOCKER" >/dev/null 2>&1; then
  if [ -x /Applications/Docker.app/Contents/Resources/bin/docker ]; then
    DOCKER=/Applications/Docker.app/Contents/Resources/bin/docker
  else
    echo "chaos-redis: docker not found (set DOCKER=/path/to/docker)" >&2
    exit 1
  fi
fi

REDIS_NAME=mall-chaos-redis
REDIS_PORT=16379

cleanup() {
  "$DOCKER" rm -f "$REDIS_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
echo ">> starting Redis"
"$DOCKER" run -d --name "$REDIS_NAME" -p "${REDIS_PORT}:6379" redis:8.10.3-alpine >/dev/null
for _ in $(seq 1 30); do
  if "$DOCKER" exec "$REDIS_NAME" redis-cli ping >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
"$DOCKER" exec "$REDIS_NAME" redis-cli ping >/dev/null

export TEST_REDIS_ADDR="127.0.0.1:${REDIS_PORT}"
export CHAOS_REDIS=1

echo ">> running admission under Redis outage"
go test ./internal/services/seckill -run TestChaosAdmissionSurvivesRedisRestart -count=1 -v &
test_pid=$!

sleep 6
echo ">> stopping Redis"
"$DOCKER" stop "$REDIS_NAME" >/dev/null
sleep 8
echo ">> restarting Redis"
"$DOCKER" start "$REDIS_NAME" >/dev/null

if wait "$test_pid"; then
  echo "chaos-redis: OK"
else
  echo "chaos-redis: FAILED" >&2
  exit 1
fi
