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

mysql_run() {
  mysql -h "${MYSQL_HOST}" -P "${MYSQL_PORT}" -u "${MYSQL_USER}" "$@"
}

mysql_value() {
  mysql_run -N -B -e "$1" "$2"
}

echo "[migrate] creating databases and DTM schema"
mysql_run < /opt/init/00-databases.sql
mysql_run < /opt/init/02-dtm.sql

for dir in /migrations/*/; do
  service="$(basename "${dir}")"
  database="mall_${service}"

  # Version + checksum ledger: applied files are never replayed, and editing a
  # file after it ran fails the job instead of silently diverging.
  mysql_run -e "CREATE TABLE IF NOT EXISTS schema_migrations (
    filename VARCHAR(255) NOT NULL,
    checksum CHAR(64) NOT NULL,
    applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (filename)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" "${database}"

  for file in "${dir}"*.sql; do
    [ -e "${file}" ] || continue
    key="${service}/$(basename "${file}")"
    sum="$(sha256sum "${file}" | cut -d' ' -f1)"
    applied="$(mysql_value "SELECT checksum FROM schema_migrations WHERE filename='${key}'" "${database}")"
    if [ -n "${applied}" ]; then
      if [ "${applied}" != "${sum}" ]; then
        echo "[migrate] ${key} changed after it was applied (checksum mismatch)" >&2
        exit 1
      fi
      echo "[migrate] ${key} already applied"
      continue
    fi
    echo "[migrate] ${database} <- ${file}"
    mysql_run "${database}" < "${file}"
    mysql_run -e "INSERT INTO schema_migrations (filename, checksum) VALUES ('${key}', '${sum}')" "${database}"
  done
done

echo "[migrate] complete"
