#!/usr/bin/env bash
# Start the local PostGIS used by the integration tests.
#
# Docker is not available in this environment, so the tests run against a
# system PostgreSQL cluster started by hand. This script makes that one command
# and is idempotent: running it against an already-running cluster is a no-op.
#
# CI uses the postgis/postgis service container instead (see ci.yml); this is
# only for local runs.
set -euo pipefail

PGBIN=/usr/lib/postgresql/16/bin
PGDATA=/var/lib/postgresql/16/main
PGCONF=/etc/postgresql/16/main/postgresql.conf
PORT=5433
export DATABASE_URL="postgres://delivery@127.0.0.1:${PORT}/delivery?sslmode=disable"

if pg_isready -h 127.0.0.1 -p "$PORT" -q 2>/dev/null; then
    echo "postgres already running on ${PORT}"
    echo "DATABASE_URL=${DATABASE_URL}"
    exit 0
fi

mkdir -p /var/run/postgresql
chown postgres:postgres /var/run/postgresql
: > /tmp/pglog.txt
chmod 666 /tmp/pglog.txt

su postgres -c "${PGBIN}/pg_ctl -D ${PGDATA} \
    -o '-p ${PORT} -c listen_addresses=127.0.0.1 -c config_file=${PGCONF}' \
    -l /tmp/pglog.txt start" >/dev/null

for _ in $(seq 1 30); do
    pg_isready -h 127.0.0.1 -p "$PORT" -q 2>/dev/null && break
    sleep 0.5
done

if ! pg_isready -h 127.0.0.1 -p "$PORT" -q 2>/dev/null; then
    echo "postgres did not start; log follows:" >&2
    tail -20 /tmp/pglog.txt >&2
    exit 1
fi

# The role and database are created once and survive restarts; PostGIS needs a
# superuser to install, so the extension is added here rather than by the
# migration running as the application role.
su postgres -c "psql -p ${PORT} -h 127.0.0.1 -tAc \
    \"SELECT 1 FROM pg_roles WHERE rolname='delivery'\"" | grep -q 1 \
    || su postgres -c "psql -p ${PORT} -h 127.0.0.1 -qc 'CREATE ROLE delivery LOGIN CREATEDB'"

su postgres -c "psql -p ${PORT} -h 127.0.0.1 -tAc \
    \"SELECT 1 FROM pg_database WHERE datname='delivery'\"" | grep -q 1 \
    || su postgres -c "psql -p ${PORT} -h 127.0.0.1 -qc 'CREATE DATABASE delivery OWNER delivery'"

su postgres -c "psql -p ${PORT} -h 127.0.0.1 -d delivery -qc 'CREATE EXTENSION IF NOT EXISTS postgis'"

echo "postgres ready on ${PORT}"
echo "DATABASE_URL=${DATABASE_URL}"
