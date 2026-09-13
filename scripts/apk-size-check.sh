#!/usr/bin/env bash
# CI check 9b — release APK must stay under the ceiling and must not grow more
# than the allowed percentage in a single PR (05-architecture.md 2.9).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CEILING_MB="${APK_CEILING_MB:-20}"
MAX_GROWTH_PCT="${APK_MAX_GROWTH_PCT:-10}"
BASELINE="${REPO_ROOT}/frontend/apk-size-baseline.txt"

FOUND=0
while IFS= read -r pubspec; do
  app_dir="$(dirname "${pubspec}")"
  app="$(basename "${app_dir}")"
  [[ -d "${app_dir}/android" ]] || continue
  FOUND=1
  echo "==> building release APK (${app})"
  ( cd "${app_dir}" && flutter build apk --release --split-per-abi )

  apk="$(find "${app_dir}/build/app/outputs" -name '*arm64-v8a*release*.apk' -print -quit)"
  [[ -n "${apk}" ]] || { echo "FAIL: no release APK produced for ${app}" >&2; exit 1; }

  bytes="$(stat -c%s "${apk}")"
  mb="$(awk -v b="${bytes}" 'BEGIN{printf "%.2f", b/1048576}')"
  echo "${app}: ${mb} MB"

  if awk -v m="${mb}" -v c="${CEILING_MB}" 'BEGIN{exit !(m+0 > c+0)}'; then
    echo "FAIL: ${app} APK ${mb} MB exceeds the ${CEILING_MB} MB ceiling" >&2
    exit 1
  fi

  prev="$(grep -E "^${app} " "${BASELINE}" 2>/dev/null | awk '{print $2}' || true)"
  if [[ -n "${prev}" ]]; then
    if awk -v m="${mb}" -v p="${prev}" -v g="${MAX_GROWTH_PCT}" \
      'BEGIN{exit !(p+0 > 0 && ((m-p)/p)*100 > g+0)}'; then
      echo "FAIL: ${app} APK grew from ${prev} MB to ${mb} MB, over the ${MAX_GROWTH_PCT}% limit." >&2
      echo "Justify the growth in the PR description and update ${BASELINE}." >&2
      exit 1
    fi
  fi
done < <(find "${REPO_ROOT}/frontend" -name pubspec.yaml -not -path '*/.*')

[[ "${FOUND}" -eq 0 ]] && echo "apk-size-check: no Android app yet; skipped"
exit 0
