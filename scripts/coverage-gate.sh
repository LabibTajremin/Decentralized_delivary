#!/usr/bin/env bash
# Runs the backend test module and fails if coverage is below the gate.
#
# Coverage is measured from the separate tests module against the backend
# module (docs/build/00-rules.md 3.1). Exclusions are applied by filtering the
# coverage profile, so the reported figure is the gated figure.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GATE="${COVERAGE_GATE:-100.0}"
BACKEND_PKGS="github.com/rootlogic-lab/delivery/backend/..."
EXCLUSIONS="${REPO_ROOT}/backend/tests/coverage-exclusions.txt"

cd "${REPO_ROOT}/backend/tests"

# -count=1 is mandatory: the architecture guard tests read files outside their
# own package, so a cached PASS can hide a real violation.
go test ./... \
  -coverpkg="${BACKEND_PKGS}" \
  -coverprofile=coverage.raw.out \
  -covermode=atomic \
  -count=1

if [[ ! -s coverage.raw.out ]]; then
  echo "coverage: no profile produced" >&2
  exit 1
fi

# Apply the exclusion list.
head -n 1 coverage.raw.out > coverage.out
EXCLUDED=0
while IFS= read -r line; do
  line="${line%%#*}"
  line="$(echo "${line}" | xargs || true)"
  [[ -z "${line}" ]] && continue
  printf '%s\n' "${line}"
done < "${EXCLUSIONS}" > /tmp/cov-excl.txt || true

if [[ -s /tmp/cov-excl.txt ]]; then
  tail -n +2 coverage.raw.out | grep -v -F -f /tmp/cov-excl.txt >> coverage.out || true
  EXCLUDED=$(( $(wc -l < coverage.raw.out) - $(wc -l < coverage.out) ))
else
  tail -n +2 coverage.raw.out >> coverage.out
fi

# Guard against a silent hole in the gate: -coverpkg only instruments packages
# that are actually linked into a test binary. A package no test ever imports
# produces no profile lines at all, so a naive total would read 100% while that
# package is completely untested. Fail instead.
MISSING=""
while IFS= read -r pkg; do
  [[ -z "${pkg}" ]] && continue
  grep -qF -- "${pkg}" /tmp/cov-excl.txt 2>/dev/null && continue
  if grep -qF -- "${pkg}/" coverage.raw.out || grep -qE "^${pkg}:" coverage.raw.out; then
    continue
  fi
  # A package of pure declarations has no statements and legitimately produces
  # no profile lines; one containing functions does not.
  dir="${REPO_ROOT}/backend/${pkg#github.com/rootlogic-lab/delivery/backend/}"
  if [[ -d "${dir}" ]] && grep -rqE '^func ' "${dir}"/*.go 2>/dev/null; then
    MISSING="${MISSING}  ${pkg}"$'\n'
  fi
done < <(cd "${REPO_ROOT}/backend" && go list ./... 2>/dev/null)

if [[ -n "${MISSING}" ]]; then
  echo "FAIL: these packages contain functions but appear in no coverage profile." >&2
  echo "No test imports them, so the gate cannot see them:" >&2
  printf '%s' "${MISSING}" >&2
  exit 1
fi

BODY_LINES=$(( $(wc -l < coverage.out) - 1 ))
if [[ "${BODY_LINES}" -le 0 ]]; then
  echo "coverage: 100.0% (no coverable statements outside the exclusion list)"
  echo "coverage: excluded ${EXCLUDED} profile lines"
  exit 0
fi

TOTAL="$(go tool cover -func=coverage.out | tail -1 | awk '{print $NF}' | tr -d '%')"
echo "coverage: ${TOTAL}% (gate ${GATE}%, excluded ${EXCLUDED} profile lines)"

if awk -v t="${TOTAL}" -v g="${GATE}" 'BEGIN{exit !(t+0 < g+0)}'; then
  echo "FAIL: coverage ${TOTAL}% is below the ${GATE}% gate" >&2
  go tool cover -func=coverage.out | awk '$NF!="100.0%"' | head -40 >&2
  exit 1
fi
echo "coverage gate passed"
