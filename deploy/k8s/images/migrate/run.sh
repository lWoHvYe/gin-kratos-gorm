#!/bin/sh
set -eu

: "${MYSQL_HOST:=mysql.mall-data.svc.cluster.local}"
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:=root}"
: "${MYSQL_PASSWORD:=root}"
: "${MIGRATIONS_ROOT:=/migrations}"
: "${MIGRATION_LOCK_SECONDS:=600}"
: "${MIGRATION_LOCK_WAIT_SECONDS:=300}"

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

echo "[migrate] creating databases"
mysql_run < /opt/init/00-databases.sql

# The lease lives in a dedicated database and survives across mysql CLI
# invocations (GET_LOCK is connection-local, but this script opens one client
# per statement). It prevents two Helm migration jobs from applying the same
# file concurrently.
# Alpine's minimal image has no `hostname`; `$$` and the epoch can be
# identical across two containers. Use a kernel-random suffix so two runners
# can never share an owner token.
OWNER="migrate-$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
LOCK_NAME="mall_migrate"
LOCK_DB="mall_migrate"

mysql_run -e "CREATE DATABASE IF NOT EXISTS ${LOCK_DB} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql_run -e "CREATE TABLE IF NOT EXISTS ${LOCK_DB}.schema_migration_lock (
  name VARCHAR(64) NOT NULL,
  owner VARCHAR(128) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  PRIMARY KEY (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;"

acquired=0
deadline=$(( $(date +%s) + MIGRATION_LOCK_WAIT_SECONDS ))
while [ "$(date +%s)" -le "${deadline}" ]; do
  mysql_run -e "INSERT INTO ${LOCK_DB}.schema_migration_lock (name, owner, expires_at)
    VALUES ('${LOCK_NAME}', '${OWNER}', DATE_ADD(NOW(3), INTERVAL ${MIGRATION_LOCK_SECONDS} SECOND))
    ON DUPLICATE KEY UPDATE
      owner = IF(expires_at < NOW(3), VALUES(owner), owner),
      expires_at = IF(expires_at < NOW(3), VALUES(expires_at), expires_at);"
  holder="$(mysql_value "SELECT owner FROM ${LOCK_DB}.schema_migration_lock WHERE name='${LOCK_NAME}'" "${LOCK_DB}")"
  if [ "${holder}" = "${OWNER}" ]; then
    acquired=1
    break
  fi
  echo "[migrate] waiting for migration lease held by ${holder}"
  sleep 2
done
if [ "${acquired}" -ne 1 ]; then
  echo "[migrate] could not acquire migration lease within ${MIGRATION_LOCK_WAIT_SECONDS}s" >&2
  exit 1
fi

release_lock() {
  mysql_run -e "DELETE FROM ${LOCK_DB}.schema_migration_lock
    WHERE name='${LOCK_NAME}' AND owner='${OWNER}'" >/dev/null 2>&1 || true
}
trap release_lock EXIT
trap 'exit 1' INT TERM

renew_lock() {
  renewed="$(mysql_run -N -B "${LOCK_DB}" -e "UPDATE ${LOCK_DB}.schema_migration_lock
    SET expires_at = DATE_ADD(NOW(3), INTERVAL ${MIGRATION_LOCK_SECONDS} SECOND)
    WHERE name='${LOCK_NAME}' AND owner='${OWNER}';
    SELECT COUNT(*) FROM ${LOCK_DB}.schema_migration_lock
    WHERE name='${LOCK_NAME}' AND owner='${OWNER}';")"
  if [ "${renewed}" != "1" ]; then
    echo "[migrate] lost the migration lease; refusing to continue" >&2
    exit 1
  fi
}

echo "[migrate] creating DTM schema"
mysql_run < /opt/init/02-dtm.sql

for dir in "${MIGRATIONS_ROOT}"/*/; do
  service="$(basename "${dir}")"
  database="mall_${service}"
  [ -d "${dir}" ] || continue

  # Version + checksum ledger: applied files are never replayed, and editing a
  # file after it ran fails the job instead of silently diverging.
  mysql_run -e "CREATE TABLE IF NOT EXISTS schema_migrations (
    filename VARCHAR(255) NOT NULL,
    checksum CHAR(64) NOT NULL,
    applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (filename)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" "${database}"

  # Attempt ledger: DDL commits implicitly in MySQL, so the runner records
  # RUNNING before executing and FAILED on error. A failed/running attempt is
  # never retried blindly; an operator must inspect the schema or restore.
  mysql_run -e "CREATE TABLE IF NOT EXISTS schema_migration_attempts (
    filename VARCHAR(255) NOT NULL,
    checksum CHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL,
    owner VARCHAR(128) NOT NULL,
    error VARCHAR(512) NOT NULL DEFAULT '',
    started_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (filename)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;" "${database}"

  for file in "${dir}"*.sql; do
    [ -e "${file}" ] || continue
    renew_lock
    key="${service}/$(basename "${file}")"
    sum="$(sha256sum "${file}" | cut -d' ' -f1)"
    applied="$(mysql_value "SELECT checksum FROM schema_migrations WHERE filename='${key}'" "${database}")"
    if [ -n "${applied}" ]; then
      if [ "${applied}" != "${sum}" ]; then
        echo "[migrate] ${key} changed after it was applied (checksum mismatch)" >&2
        exit 1
      fi
      # A crash after the ledger insert but before the attempt update leaves
      # RUNNING with an applied ledger; that is safe to reconcile as APPLIED.
      mysql_run -e "UPDATE schema_migration_attempts
        SET status='APPLIED', error='', updated_at=NOW(3)
        WHERE filename='${key}' AND status='RUNNING'" "${database}"
      echo "[migrate] ${key} already applied"
      continue
    fi

    attempt_status="$(mysql_value "SELECT status FROM schema_migration_attempts WHERE filename='${key}'" "${database}")"
    if [ -n "${attempt_status}" ]; then
      echo "[migrate] ${key} has a previous ${attempt_status} attempt without an applied ledger" >&2
      echo "[migrate] refusing to replay; inspect ${database}.schema_migration_attempts or restore the backup" >&2
      exit 1
    fi

    if ! mysql_run -e "INSERT INTO schema_migration_attempts
      (filename, checksum, status, owner, error, started_at, updated_at)
      VALUES ('${key}', '${sum}', 'RUNNING', '${OWNER}', '', NOW(3), NOW(3));" "${database}"; then
      echo "[migrate] could not record attempt for ${key}" >&2
      exit 1
    fi

    echo "[migrate] ${database} <- ${file}"
    if ! mysql_run "${database}" < "${file}"; then
      mysql_run -e "UPDATE schema_migration_attempts
        SET status='FAILED', error='SQL failed; see migrate job log', updated_at=NOW(3)
        WHERE filename='${key}' AND status='RUNNING' AND owner='${OWNER}'" "${database}" || true
      echo "[migrate] ${key} failed; attempt recorded as FAILED" >&2
      exit 1
    fi

    if ! mysql_run -e "START TRANSACTION;
      INSERT INTO schema_migrations (filename, checksum) VALUES ('${key}', '${sum}');
      UPDATE schema_migration_attempts
        SET status='APPLIED', error='', updated_at=NOW(3)
        WHERE filename='${key}' AND status='RUNNING' AND owner='${OWNER}';
      COMMIT;" "${database}"; then
      echo "[migrate] ${key} applied but ledger update failed; inspect before retrying" >&2
      exit 1
    fi
  done
done

echo "[migrate] complete"
