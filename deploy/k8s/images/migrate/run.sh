#!/bin/sh
set -eu

: "${MYSQL_HOST:=mysql.mall-data.svc.cluster.local}"
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:=root}"
: "${MYSQL_PASSWORD:=root}"

export MYSQL_PWD="${MYSQL_PASSWORD}"

until mysqladmin ping -h "${MYSQL_HOST}" -P "${MYSQL_PORT}" -u "${MYSQL_USER}" --silent; do
  echo "[migrate] waiting for mysql ${MYSQL_HOST}:${MYSQL_PORT}"
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
