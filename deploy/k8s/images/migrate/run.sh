#!/bin/sh
set -eu

: "${MYSQL_HOST:=mysql.mall-data.svc.cluster.local}"
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:=root}"
: "${MYSQL_PASSWORD:=root}"

export MYSQL_PWD="${MYSQL_PASSWORD}"

attempt=0
until mysqladmin ping -h "${MYSQL_HOST}" -P "${MYSQL_PORT}" -u "${MYSQL_USER}" --silent; do
  attempt=$((attempt + 1))
  if [ "${attempt}" -ge 150 ]; then
    echo "[migrate] mysql did not become ready in 5 minutes" >&2
    exit 1
  fi
  echo "[migrate] waiting for mysql ${MYSQL_HOST}:${MYSQL_PORT} (${attempt}/150)"
  sleep 2
done

echo "[migrate] creating databases and DTM schema"
mysql -h "${MYSQL_HOST}" -P "${MYSQL_PORT}" -u "${MYSQL_USER}" < /opt/init/00-databases.sql
mysql -h "${MYSQL_HOST}" -P "${MYSQL_PORT}" -u "${MYSQL_USER}" < /opt/init/02-dtm.sql

for dir in /migrations/*/; do
  service="$(basename "${dir}")"
  database="mall_${service}"
  for file in "${dir}"*.sql; do
    [ -e "${file}" ] || continue
    echo "[migrate] ${database} <- ${file}"
    mysql -h "${MYSQL_HOST}" -P "${MYSQL_PORT}" -u "${MYSQL_USER}" "${database}" < "${file}"
  done
done

echo "[migrate] complete"
