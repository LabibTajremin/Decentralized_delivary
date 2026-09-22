#!/usr/bin/env bash
# Start the local Redis used by the auth integration tests.
#
# Docker is not available in this environment, so Redis runs as a plain process.
# Idempotent: running it against an already-running server is a no-op.
#
# CI uses the redis:7-alpine service container instead (see ci.yml).
set -euo pipefail

PORT=6379
export REDIS_URL="redis://127.0.0.1:${PORT}/0"

if redis-cli -p "$PORT" ping >/dev/null 2>&1; then
    echo "redis already running on ${PORT}"
    echo "REDIS_URL=${REDIS_URL}"
    exit 0
fi

mkdir -p /var/lib/redis-dev
# appendonly is on so a restart does not silently drop sessions mid-run, which
# would look like spurious token failures rather than a restarted server.
redis-server \
    --port "$PORT" \
    --bind 127.0.0.1 \
    --daemonize yes \
    --dir /var/lib/redis-dev \
    --appendonly yes \
    --logfile /var/lib/redis-dev/redis.log

for _ in $(seq 1 30); do
    redis-cli -p "$PORT" ping >/dev/null 2>&1 && break
    sleep 0.2
done

if ! redis-cli -p "$PORT" ping >/dev/null 2>&1; then
    echo "redis did not start; log follows:" >&2
    tail -20 /var/lib/redis-dev/redis.log >&2
    exit 1
fi

echo "redis ready on ${PORT}"
echo "REDIS_URL=${REDIS_URL}"
