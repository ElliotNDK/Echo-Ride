#!/usr/bin/env bash
set -uo pipefail
GW=${GW:-http://localhost:9080}
SECRET=${JWT_SECRET:?set JWT_SECRET}
KEY=${JWT_KEY:-echo-ride}          # must equal the consumer key in apisix.yaml
fail=0

b64() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
mint() { # mint <secret> <sub> <role>
  local h p s
  h=$(printf '{"alg":"HS256","typ":"JWT"}' | b64)
  p=$(printf '{"key":"%s","sub":"%s","role":"%s","exp":%s}' \
        "$KEY" "$2" "$3" "$(( $(date +%s) + 600 ))" | b64)
  s=$(printf '%s.%s' "$h" "$p" | openssl dgst -binary -sha256 -hmac "$1" | b64)
  printf '%s.%s.%s' "$h" "$p" "$s"
}
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
check() { # check <name> <expected> <actual>
  if [[ "$2" == "$3" ]]; then echo "PASS $1"; else echo "FAIL $1 (want $2, got $3)"; fail=1; fi
}

TOKEN=$(mint "$SECRET" 42 rider)
BAD=$(mint "wrong-secret" 42 rider)

check "healthz"                200 "$(code $GW/healthz)"
check "no token"               401 "$(code $GW/api/v1/rides)"
check "garbage token"          401 "$(code -H 'Authorization: Bearer bad' $GW/api/v1/rides)"
check "wrong-secret token"     401 "$(code -H "Authorization: Bearer $BAD" $GW/api/v1/rides)"
check "valid token reaches upstream" 200 "$(code -H "Authorization: Bearer $TOKEN" $GW/api/v1/rides)"
check "public login needs no token"  200 "$(code -X POST $GW/api/v1/auth/login)"

# identity injection: forged header must be replaced by the token's claims
body=$(curl -s -H "Authorization: Bearer $TOKEN" -H 'X-User-Id: 999' -H 'X-User-Role: admin' $GW/api/v1/rides)
grep -qi '^X-User-Id: 42'    <<<"$body" && echo "PASS X-User-Id from token"   || { echo "FAIL X-User-Id"; fail=1; }
grep -qi '^X-User-Role: rider' <<<"$body" && echo "PASS X-User-Role from token" || { echo "FAIL X-User-Role"; fail=1; }
grep -qi '999' <<<"$body" && { echo "FAIL forged id leaked"; fail=1; }

# public routes must strip client-supplied identity
pub=$(curl -s -X POST -H 'X-User-Id: 999' $GW/api/v1/auth/login)
grep -qi 'X-User-Id' <<<"$pub" && { echo "FAIL public route forwards X-User-Id"; fail=1; } || echo "PASS public route strips X-User-Id"

# rate limit on public auth routes (limit-count 20/60s)
last=200; for _ in $(seq 25); do last=$(code -X POST $GW/api/v1/auth/login); done
check "login rate limit" 429 "$last"

exit $fail