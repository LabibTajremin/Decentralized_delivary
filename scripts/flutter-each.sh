#!/usr/bin/env bash
# Runs a flutter command in every app package under frontend/.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CMD="${1:?usage: flutter-each.sh <flutter-subcommand>}"
shift || true

FOUND=0
while IFS= read -r pubspec; do
  FOUND=1
  app_dir="$(dirname "${pubspec}")"
  echo "==> flutter ${CMD} (${app_dir})"
  ( cd "${app_dir}" && flutter pub get >/dev/null && flutter "${CMD}" "$@" )
done < <(find "${REPO_ROOT}/frontend" -name pubspec.yaml -not -path '*/.*')

[[ "${FOUND}" -eq 0 ]] && echo "flutter-each: no Flutter apps yet; skipped"
exit 0
