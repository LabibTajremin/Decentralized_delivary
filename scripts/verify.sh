#!/usr/bin/env bash
# Every gate CI runs, in one command, in CI's order.
#
# A phase is not finished until this exits 0. It exists so that finishing a
# phase is a thing you *check* rather than a thing you remember: a session that
# forgets one gate ships a red branch, and the gate it forgets is usually the
# one that would have caught the bug.
#
# It brings the local services up first — Postgres and Redis do not survive a
# container restart, and every "the database is refusing connections" detour so
# far has been that and nothing else.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

export DATABASE_URL="${DATABASE_URL:-postgres://delivery@127.0.0.1:5433/delivery?sslmode=disable}"
export REDIS_URL="${REDIS_URL:-redis://127.0.0.1:6379/0}"

failed=()
step() {
  local name="$1"; shift
  printf '\n\033[1m── %s\033[0m\n' "${name}"
  if "$@"; then
    printf '\033[32mOK\033[0m  %s\n' "${name}"
  else
    printf '\033[31mFAIL\033[0m  %s\n' "${name}"
    failed+=("${name}")
  fi
}

# Services first, quietly. Both scripts are idempotent.
./scripts/dev-postgres.sh >/dev/null 2>&1 || true
./scripts/dev-redis.sh    >/dev/null 2>&1 || true

step "go build"            bash -c 'cd backend && go build ./...'
step "go vet"              bash -c 'cd backend && go vet ./...'
step "golangci-lint"       bash -c 'cd backend && golangci-lint run ./...'
step "gofmt (test module)" bash -c '
    cd backend/tests
    unformatted="$(gofmt -l .)"
    if [[ -n "${unformatted}" ]]; then echo "${unformatted}"; exit 1; fi'
step "architecture lint"   bash -c '
    cd backend/tests
    go test ./unit/... -count=1 \
      -run "TestDomainImportsNothingInternal|TestApplicationImportsDomainOnly|TestTransportDoesNotImportInfrastructure|TestCrossModuleImportsGoThroughExternal"'
step "OpenAPI matches the served routes" bash -c 'cd backend/tests && go test ./unit/openapi/... -count=1'
step "migrations up and down" ./scripts/migrate-check.sh
step "tests and coverage gate" ./scripts/coverage-gate.sh
step "thin-client lint"    ./scripts/thin-client-lint.sh
# Runs whether or not any Dart exists yet: it proves the lint by feeding it
# violations of its own, so it is meaningful before the first screen is built.
step "thin-client lint self-test" ./scripts/thin-client-lint-selftest.sh

if [[ -d frontend && -f frontend/pubspec.yaml ]]; then
  step "flutter analyze"       bash -c 'cd frontend && flutter analyze'
  step "flutter coverage gate" ./scripts/flutter-coverage-gate.sh
  step "APK size budget"       ./scripts/apk-size-check.sh
fi

printf '\n'
if (( ${#failed[@]} == 0 )); then
  printf '\033[32mverify: every gate passed. The phase may be committed.\033[0m\n'
  exit 0
fi
printf '\033[31mverify: %d gate(s) failed:\033[0m\n' "${#failed[@]}"
printf '  - %s\n' "${failed[@]}"
printf 'The phase is not finished. Do not commit it as done.\n'
exit 1
