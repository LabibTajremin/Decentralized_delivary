#!/usr/bin/env bash
# Proves that scripts/thin-client-lint.sh actually fails on the things it
# forbids, by feeding it each one on purpose.
#
# A gate nobody has watched fail is not known to work. The thin-client lint is
# the only automated defence for rule 2.9 — every business rule lives in Go —
# and it guards a codebase that does not yet contain a single screen. If its
# patterns were subtly wrong, nothing would notice until P18 had written
# thousands of lines under a lint that silently passed everything.
#
# So this runs before any screen exists, which is what P17's acceptance asks
# for, and keeps running afterwards.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LINT="${REPO_ROOT}/scripts/thin-client-lint.sh"
FIXTURES="${REPO_ROOT}/frontend/.thin-client-lint-selftest"

cleanup() { rm -rf "${FIXTURES}"; }
trap cleanup EXIT
cleanup
mkdir -p "${FIXTURES}"

FAILED=0

# Runs the lint with one fixture file in place and checks the outcome.
#   expect_reject <name> <dart source>
#   expect_accept <name> <dart source>
run_with_fixture() {
  local source="$1"
  printf '%s\n' "${source}" > "${FIXTURES}/fixture.dart"
  "${LINT}" >/dev/null 2>&1
  local status=$?
  rm -f "${FIXTURES}/fixture.dart"
  return "${status}"
}

expect_reject() {
  local name="$1" source="$2"
  if run_with_fixture "${source}"; then
    echo "FAIL: the lint accepted ${name}, which 2.9 forbids" >&2
    echo "      offending source: ${source}" >&2
    FAILED=1
  else
    echo "ok: rejected ${name}"
  fi
}

expect_accept() {
  local name="$1" source="$2"
  if run_with_fixture "${source}"; then
    echo "ok: accepted ${name}"
  else
    echo "FAIL: the lint rejected ${name}, which is legal" >&2
    echo "      offending source: ${source}" >&2
    FAILED=1
  fi
}

# 1. Money or distance used as an operand.
expect_reject 'a fee added to something' \
  'void f() { print(deliveryFee + serviceCharge); }'
expect_reject 'a distance scaled' \
  'void f() { print(distanceKm * 2); }'

# 2. Arithmetic assigned to a money or distance name.
expect_reject 'a total worked out in the client' \
  'void f() { final total = a + b; }'
expect_reject 'a radius worked out in the client' \
  'void f() { final baseRadius = step * 2; }'

# 3. An Appendix B threshold hardcoded in the app.
expect_reject 'a COD limit baked into the app' \
  'void f() { const codLimit = 500000; }'
expect_reject 'a free-delivery threshold baked into the app' \
  'void f() { const freeDeliveryThreshold = 100000; }'

# And the other direction: rendering what the server computed is the whole
# point of the client, so none of this may trip the lint.
expect_accept 'painting a preformatted money string' \
  'void f(Money m) { Text(m.display); }'
expect_accept 'reading a minor unit without computing on it' \
  'void f(Money m) { expect(m.minor, 25000); }'
expect_accept 'a layout number that has nothing to do with money' \
  'void f() { final gap = 8 + 4; }'

if [[ "${FAILED}" -ne 0 ]]; then
  echo "" >&2
  echo "thin-client lint self-test failed: the lint does not enforce 2.9 the" >&2
  echo "way it claims to. Fix scripts/thin-client-lint.sh before trusting it." >&2
  exit 1
fi
echo "thin-client lint self-test passed"
