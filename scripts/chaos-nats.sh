#!/usr/bin/env bash
# NATS outage chaos for the outbox relay.
#
# Starts isolated MySQL and JetStream NATS containers, runs the relay test in the
# background, stops NATS mid-flight, restarts it, and requires every event to be
# published without dead-lettering. Uses dedicated names/ports so a shared
# Docker stack is not touched.
set -euo pipefail
cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-docker}"
if ! command -v "$DOCKER" >/dev/null 2>&1; then
  if [ -x /Applications/Docker.app/Contents/Resources/bin/docker ]; then
    DOCKER=/Applications/Docker.app/Contents/Resources/bin/docker
  else
    echo "chaos-nats: docker not found (set DOCKER=/path/to/docker)" >&2
    exit 1
  fi
fi

MYSQL_NAME=mall-chaos-mysql
NATS_NAME=mall-chaos-nats
MYSQL_PORT=13306
NATS_PORT=14222

cleanup() {
  "$DOCKER" rm -f "$MYSQL_NAME" "$NATS_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup
echo ">> starting MySQL"
"$DOCKER" run -d --name "$MYSQL_NAME" -p "${MYSQL_PORT}:3306" \
  -e MYSQL_ROOT_PASSWORD=root mysql:8.4 >/dev/null
# mysqladmin can answer during the temporary init server; wait for a real query.
for _ in $(seq 1 90); do
  if "$DOCKER" exec "$MYSQL_NAME" mysql -uroot -proot -e 'SELECT 1' >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
"$DOCKER" exec "$MYSQL_NAME" mysql -uroot -proot -e 'SELECT 1' >/dev/null
sleep 2

echo ">> starting NATS JetStream"
"$DOCKER" run -d --name "$NATS_NAME" -p "${NATS_PORT}:4222" nats:2.10-alpine -js >/dev/null
for _ in $(seq 1 30); do
  if nc -z 127.0.0.1 "$NATS_PORT" 2>/dev/null; then
    break
  fi
  sleep 1
done
nc -z 127.0.0.1 "$NATS_PORT"

export TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:${MYSQL_PORT})/mall_outbox_test?parseTime=true"
export TEST_NATS_URL="nats://127.0.0.1:${NATS_PORT}"
export CHAOS_NATS=1

echo ">> running relay under NATS outage"
go test ./internal/platform/outbox -run TestChaosRelaySurvivesNatsOutage -count=1 -v &
test_pid=$!

# The test inserts events and waits 8s before relaying; stop NATS inside that
# window, then bring it back after the relay has observed failures.
sleep 7
echo ">> stopping NATS"
"$DOCKER" stop "$NATS_NAME" >/dev/null
sleep 10
echo ">> restarting NATS"
"$DOCKER" start "$NATS_NAME" >/dev/null

if wait "$test_pid"; then
  echo "chaos-nats: OK"
else
  echo "chaos-nats: FAILED" >&2
  exit 1
fi
