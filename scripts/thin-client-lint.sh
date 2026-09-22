#!/usr/bin/env bash
# Fails if Flutter code performs money or distance arithmetic, or hardcodes a
# business threshold. See docs/build/05-architecture.md section 2.9.
#
# The client renders what the server computed. Any rule living in the app needs
# an app-store release to change, which this product cannot rely on: rural users
# on low-end devices update rarely, and these rules are admin-tunable per area.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND="${REPO_ROOT}/frontend"
FAILED=0

if ! find "${FRONTEND}" -name '*.dart' -print -quit 2>/dev/null | grep -q .; then
  echo "thin-client lint: no Dart sources yet, nothing to check"
  exit 0
fi

# Generated sources are exempt: they are produced from the API contract.
filter_generated() {
  grep -v '\.g\.dart:' | grep -v '\.freezed\.dart:' | grep -v '\.gr\.dart:'
}

report() {
  local label="$1" hits="$2"
  if [[ -n "${hits}" ]]; then
    echo "FAIL: ${label}" >&2
    echo "${hits}" >&2
    FAILED=1
  fi
}

# Matches an identifier whose name carries money or distance meaning, in either
# lower_case or camelCase (deliveryFee, itemTotal, price, distanceKm). The
# camelCase branch requires the capitalised form so that words merely containing
# a token — "coffee" — are not reported.
MONEY_WORDS_LOWER='price|amount|total|subtotal|fee|charge|discount|vat|tax|distance|radius|km'
MONEY_WORDS_CAMEL='Price|Amount|Total|Subtotal|Fee|Charge|Discount|Vat|Tax|Distance|Radius|Km'
MONEY="([a-zA-Z_]*(${MONEY_WORDS_CAMEL})|(${MONEY_WORDS_LOWER}))"

# 1. A money or distance identifier used as an operand: total + fee, distance * rate.
HITS="$(grep -rnE --include='*.dart' "\b${MONEY}[A-Za-z_]*[[:space:]]*[-+*/][[:space:]]*[A-Za-z0-9_.]" \
  "${FRONTEND}" 2>/dev/null | filter_generated || true)"
report "money/distance arithmetic in Flutter (2.9)" "${HITS}"

# 2. An arithmetic expression assigned to a money or distance identifier:
#    `final total = a + b;`. Lines containing a string literal are skipped so
#    that interpolating a preformatted money string stays legal.
HITS="$(grep -rnE --include='*.dart' "\b${MONEY}[A-Za-z_]*[[:space:]]*=[^=]*[-+*/]" \
  "${FRONTEND}" 2>/dev/null | filter_generated | grep -vE "[\"']" || true)"
report "money/distance computed in Flutter (2.9)" "${HITS}"

# 3. A business threshold hardcoded in the client. Every one of these is an
#    Appendix B config key that the admin tunes per area at runtime.
HITS="$(grep -rnE --include='*.dart' \
  '\b(cod_limit|codLimit|minOrder|min_order|baseRadius|base_radius|expansionStep|expansion_step|maxExpansions|max_expansions|commissionRate|commission_rate|deliveryBase|delivery_base|deliveryPerKm|delivery_per_km|freeDeliveryThreshold|free_delivery_threshold|expansionMultiplier|expansion_multiplier)\b[[:space:]]*=' \
  "${FRONTEND}" 2>/dev/null | filter_generated || true)"
report "business threshold hardcoded in Flutter (2.9)" "${HITS}"

if [[ "${FAILED}" -ne 0 ]]; then
  echo "" >&2
  echo "thin-client lint failed. Move the rule to the backend and return the" >&2
  echo "computed value in the API response (05-architecture.md 2.9)." >&2
  exit 1
fi
echo "thin-client lint passed"
