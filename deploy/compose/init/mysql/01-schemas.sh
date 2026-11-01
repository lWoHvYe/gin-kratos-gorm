#!/bin/bash
# Apply every /migrations/<service>/*.sql file to the matching mall_<service>
# database. Runs once from the MySQL entrypoint on an empty data volume, after
# 00-databases.sql.
#
# The server is socket-only during the init phase, hence --protocol=socket.
# Manual re-run:
#   docker compose -f deploy/compose/infra.yml exec mysql /docker-entrypoint-initdb.d/01-schemas.sh

set -euo pipefail

MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-root}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

# MYSQL_PWD avoids "password on the command line" warnings and keeps the
# password out of the process list.
export MYSQL_PWD="${MYSQL_ROOT_PASSWORD}"

MYSQL=(mysql -uroot --protocol=socket --default-character-set=utf8mb4)

if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
  echo "[schemas] ${MIGRATIONS_DIR} is not mounted; nothing to apply"
  exit 0
fi

shopt -s nullglob

applied=0
for service_dir in "${MIGRATIONS_DIR}"/*/; do
  service="$(basename "${service_dir}")"
  database="mall_${service}"
  files=("${service_dir}"*.sql)

  if [[ ${#files[@]} -eq 0 ]]; then
    echo "[schemas] ${service}: no .sql files, skipping"
    continue
  fi

  echo "[schemas] ${database}: ensuring database exists"
  "${MYSQL[@]}" -e "CREATE DATABASE IF NOT EXISTS \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

  # Lexicographic order is the migration order: 0001_, 0002_, ...
  while IFS= read -r file; do
    echo "[schemas] ${database} <- ${file}"
    "${MYSQL[@]}" --database="${database}" < "${file}"
    applied=$((applied + 1))
  done < <(printf '%s\n' "${files[@]}" | sort -V)
done

echo "[schemas] applied ${applied} migration file(s)"
