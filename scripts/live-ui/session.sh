# shellcheck shell=bash
# Sourced by the live passes (sync-e2e.sh, storage-e2e.sh): signs in to nosd
# at $BASE as $USER_NAME / $USER_PASS and defines
#   api <method> <path> [json body]   (fails on an HTTP error)
#   api_status <method> <path> [json body] (prints the status; body in $API_BODY_FILE)
# The session cookies are Secure; curl won't send those over http, so they
# are passed by hand. SESSION_INSECURE=1 accepts a self-signed certificate.

session_headers="$(curl ${SESSION_INSECURE:+-k} -fsS -D - -o /dev/null -H 'Content-Type: application/json' \
	-d "$(jq -nc --arg u "$USER_NAME" --arg p "$USER_PASS" '{username:$u,password:$p}')" "$BASE/api/v1/auth/login")"
session_cookies="$(printf '%s\n' "$session_headers" | tr -d '\r' | sed -n 's/^[Ss]et-[Cc]ookie: *\([^;]*\).*/\1/p' | paste -sd ';' -)"
session_csrf="$(printf '%s\n' "$session_cookies" | tr ';' '\n' | sed -n 's/^ *nos_csrf=//p')"
[ -n "$session_csrf" ] || { echo "no session from the login" >&2; exit 1; }
API_BODY_FILE="$(mktemp)"

api() {
	curl ${SESSION_INSECURE:+-k} -fsS -X "$1" -H "Cookie: $session_cookies" -H "X-CSRF-Token: $session_csrf" -H 'Content-Type: application/json' \
		${3:+-d "$3"} "$BASE$2"
}

api_status() {
	curl ${SESSION_INSECURE:+-k} -sS -o "$API_BODY_FILE" -w '%{http_code}' -X "$1" -H "Cookie: $session_cookies" -H "X-CSRF-Token: $session_csrf" \
		-H 'Content-Type: application/json' ${3:+-d "$3"} "$BASE$2"
}
