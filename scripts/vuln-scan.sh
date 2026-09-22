#!/usr/bin/env bash
# govulncheck over both Go modules.
#
# Not part of ./scripts/verify.sh, and that is deliberate. verify.sh has to
# work on a container with no outbound network — govulncheck downloads the Go
# vulnerability database on every run, and a gate that fails when the network
# is down is a gate that teaches people to ignore it.
#
# So it runs in CI (`.github/workflows/ci.yml`, the `security` job), where the
# network is a given, and by hand before a release. It is reported in
# docs/security-review.md, which records what it said and what was fixed.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOL="golang.org/x/vuln/cmd/govulncheck@latest"

status=0
for module in backend backend/tests; do
  printf '\n\033[1m── govulncheck %s\033[0m\n' "${module}"
  if ! (cd "${REPO_ROOT}/${module}" && go run "${TOOL}" ./...); then
    status=1
  fi
done

if [[ "${status}" -ne 0 ]]; then
  echo
  echo "FAIL: govulncheck found a vulnerability this code can reach." >&2
  echo "Bump the module it names (go get <module>@<fixed version> && go mod tidy)," >&2
  echo "then record it in docs/security-review.md." >&2
fi
exit "${status}"
