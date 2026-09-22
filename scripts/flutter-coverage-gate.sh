#!/usr/bin/env bash
# CI check 10 — Flutter tests with a 100% coverage gate, mirroring the backend.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GATE="${FLUTTER_COVERAGE_GATE:-100.0}"
EXCLUSIONS="${REPO_ROOT}/frontend/coverage-exclusions.txt"

FOUND=0
while IFS= read -r pubspec; do
  FOUND=1
  app_dir="$(dirname "${pubspec}")"
  echo "==> flutter test --coverage (${app_dir})"
  ( cd "${app_dir}" && flutter pub get >/dev/null && flutter test --coverage )

  lcov="${app_dir}/coverage/lcov.info"
  [[ -f "${lcov}" ]] || { echo "FAIL: no lcov.info in ${app_dir}" >&2; exit 1; }

  if [[ -f "${EXCLUSIONS}" ]]; then
    while IFS= read -r pattern; do
      pattern="${pattern%%#*}"; pattern="$(echo "${pattern}" | xargs || true)"
      [[ -z "${pattern}" ]] && continue
      lcov --remove "${lcov}" "${pattern}" -o "${lcov}" >/dev/null 2>&1 || true
    done < "${EXCLUSIONS}"
  fi

  pct="$(awk -F: '/^LF:/{f+=$2} /^LH:/{h+=$2} END{if(f==0){print "100.0"}else{printf "%.1f", (h/f)*100}}' "${lcov}")"
  echo "coverage (${app_dir}): ${pct}% (gate ${GATE}%)"
  if awk -v p="${pct}" -v g="${GATE}" 'BEGIN{exit !(p+0 < g+0)}'; then
    echo "FAIL: ${app_dir} coverage ${pct}% is below the ${GATE}% gate" >&2
    exit 1
  fi
done < <(find "${REPO_ROOT}/frontend" -name pubspec.yaml -not -path '*/.*')

[[ "${FOUND}" -eq 0 ]] && echo "flutter-coverage-gate: no Flutter apps yet; skipped"
exit 0
