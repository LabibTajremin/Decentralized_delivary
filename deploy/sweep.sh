#!/bin/sh
# The dispatch heartbeat.
#
# There is no background worker inside the API. Something has to call
# POST /v1/admin/dispatch/sweep, which does two passes: expire the offers
# nobody answered, then offer the jobs sitting on the board to somebody.
# Without the first, a job stalls behind a rider who pocketed their phone.
# Without the second, a declined job waits forever while the food goes cold.
#
# Signing in is the awkward part, because the admin surface needs a token and
# tokens come from a one-time code. Two ways through, tried in this order:
#
#   1. A refresh token in ${STATE_DIR}/refresh. Refresh tokens rotate on every
#      use, so the new one is written back immediately; losing it means
#      falling back to (2). This is the path a real deployment uses, with the
#      first token put there by hand — see docs/deployment.md.
#
#   2. DEMO_MODE, where the code comes back in the response to the request
#      that asked for it. Fully automatic, and available nowhere else.
#
# Deliberately dependency-free: busybox wget and sed, because the alternative
# is a container with curl and jq in it to make one request every twelve
# seconds.
set -u

API="${GOKLAY_API:-http://api:8080}"
INTERVAL="${SWEEP_INTERVAL:-12}"
STATE_DIR="${STATE_DIR:-/state}"
REFRESH_FILE="${STATE_DIR}/refresh"
PHONE="${ADMIN_PHONE:-}"

mkdir -p "${STATE_DIR}"

log() { echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') sweep: $*"; }

# post <path> <body> [bearer] — prints the body, or nothing on a failure.
post() {
	if [ -n "${3:-}" ]; then
		wget -q -O - --header='Content-Type: application/json' \
			--header="Authorization: Bearer $3" \
			--post-data="$2" "${API}$1" 2>/dev/null
	else
		wget -q -O - --header='Content-Type: application/json' \
			--post-data="$2" "${API}$1" 2>/dev/null
	fi
}

# field <json> <name> — the first string value of a top-level field.
field() {
	echo "$1" | sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

ACCESS=""

# sign_in fills ACCESS, and writes a refresh token for next time.
sign_in() {
	ACCESS=""

	if [ -s "${REFRESH_FILE}" ]; then
		body=$(post /v1/auth/refresh \
			"{\"refresh_token\":\"$(cat "${REFRESH_FILE}")\"}")
		ACCESS=$(field "${body}" access_token)
		if [ -n "${ACCESS}" ]; then
			# Single use: the token just spent is dead, and the new one has
			# to be kept or the next run starts over.
			field "${body}" refresh_token > "${REFRESH_FILE}"
			return 0
		fi
		log "the stored refresh token was refused; starting over"
		: > "${REFRESH_FILE}"
	fi

	if [ -z "${PHONE}" ]; then
		log "no refresh token and no ADMIN_PHONE — cannot sign in"
		return 1
	fi

	challenge=$(post /v1/auth/otp/request "{\"phone\":\"${PHONE}\"}")
	code=$(field "${challenge}" demo_code)
	if [ -z "${code}" ]; then
		log "no code was revealed for ${PHONE}. Outside DEMO_MODE this job"
		log "needs a refresh token in ${REFRESH_FILE} — see docs/deployment.md"
		return 1
	fi

	body=$(post /v1/auth/otp/verify \
		"{\"phone\":\"${PHONE}\",\"code\":\"${code}\",\"role\":\"admin\",\"device\":\"dispatch sweep\"}")
	ACCESS=$(field "${body}" access_token)
	if [ -z "${ACCESS}" ]; then
		log "sign-in failed for ${PHONE}"
		return 1
	fi
	field "${body}" refresh_token > "${REFRESH_FILE}"
	log "signed in as ${PHONE}"
}

log "starting: every ${INTERVAL}s against ${API}"

while true; do
	if [ -z "${ACCESS}" ]; then
		sign_in || { sleep "${INTERVAL}"; continue; }
	fi

	out=$(post /v1/admin/dispatch/sweep '{}' "${ACCESS}")
	if [ -z "${out}" ]; then
		# Most often the fifteen-minute access token expired. Anything else
		# that fails also gets one free re-sign-in before it is reported.
		ACCESS=""
		sign_in || log "sweep failed and re-signing in failed"
	fi

	sleep "${INTERVAL}"
done
