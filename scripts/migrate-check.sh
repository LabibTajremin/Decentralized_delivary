#!/usr/bin/env bash
# CI check 7 — every migration must apply and roll back cleanly.
# Migrations are plain SQL pairs: NNNN_name.up.sql / NNNN_name.down.sql.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIR="${REPO_ROOT}/backend/migrations"
DB_URL="${DATABASE_URL:-postgres://delivery:delivery@localhost:5432/delivery?sslmode=disable}"

shopt -s nullglob
UP=( "${DIR}"/*.up.sql )
if [[ ${#UP[@]} -eq 0 ]]; then
  echo "migrate-check: no migrations yet (the geo schema arrives in P02); nothing to verify"
  exit 0
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "migrate-check: psql not installed" >&2
  exit 1
fi

for up in "${UP[@]}"; do
  down="${up%.up.sql}.down.sql"
  if [[ ! -f "${down}" ]]; then
    echo "FAIL: ${up} has no matching .down.sql" >&2
    exit 1
  fi
done

# Start from a clean slate. CI runs against a fresh database, but a developer
# re-running this locally would otherwise fail on "relation already exists",
# which says nothing about whether the migration is correct.
for (( i=${#UP[@]}-1; i>=0; i-- )); do
  psql "${DB_URL}" -f "${UP[$i]%.up.sql}.down.sql" >/dev/null 2>&1 || true
done

echo "migrate-check: applying ${#UP[@]} migration(s)"
for up in "${UP[@]}"; do psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${up}" >/dev/null; done

echo "migrate-check: rolling back"
for (( i=${#UP[@]}-1; i>=0; i-- )); do
  down="${UP[$i]%.up.sql}.down.sql"
  psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${down}" >/dev/null
done

# This check applies the SQL directly rather than through the migrator, so
# schema_migrations knows nothing about what just happened. Clearing it matters
# on a developer's own database: without this, the rollback above leaves the
# tables gone and the ledger still claiming they exist, and the next
# `migrate up` skips every migration and then fails on the first one that
# references a table that is no longer there. CI runs against a fresh database
# where it is a harmless no-op.
psql "${DB_URL}" -c "DELETE FROM schema_migrations" >/dev/null 2>&1 || true

echo "migrate-check: up and down both clean"
