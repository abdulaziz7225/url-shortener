#!/usr/bin/env bash
# End-to-end smoke test against a running stack (make up).
# Exercises: create, redirect, metadata, custom alias, alias conflict,
# validation errors, expiry, unknown codes, and rate limiting.
set -uo pipefail

GATEWAY="${GATEWAY_URL:-http://localhost:8080}"
PASS=0
FAIL=0

green() { printf '\033[32m%s\033[0m\n' "$1"; }
red() { printf '\033[31m%s\033[0m\n' "$1"; }

check() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    green "PASS: $name (got $actual)"
    PASS=$((PASS + 1))
  else
    red "FAIL: $name (expected $expected, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

json_field() { python3 -c "import sys,json;print(json.load(sys.stdin).get('$1',''))"; }

wait_for_gateway() {
  for _ in $(seq 1 30); do
    if [[ "$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/")" == "200" ]]; then
      return 0
    fi
    sleep 1
  done
  red "gateway did not become ready at $GATEWAY"
  exit 1
}

echo "==> waiting for gateway at $GATEWAY"
wait_for_gateway

echo "==> 1. create a short URL"
LONG="https://example.com/e2e/$RANDOM"
BODY=$(curl -s -X POST "$GATEWAY/api/v1/urls" -H 'Content-Type: application/json' \
  -d "{\"long_url\":\"$LONG\"}")
CODE=$(echo "$BODY" | json_field code)
[[ -n "$CODE" ]] && green "PASS: created code=$CODE" && PASS=$((PASS + 1)) || {
  red "FAIL: no code in response: $BODY"
  FAIL=$((FAIL + 1))
}

echo "==> 2. redirect resolves to the long URL"
read -r STATUS LOCATION < <(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' "$GATEWAY/$CODE")
check "redirect status" "302" "$STATUS"
check "redirect location" "$LONG" "$LOCATION"

echo "==> 3. metadata endpoint"
META_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/api/v1/urls/$CODE")
check "metadata status" "200" "$META_STATUS"

echo "==> 4. custom alias"
ALIAS="e2e-alias-$RANDOM"
ALIAS_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/api/v1/urls" \
  -H 'Content-Type: application/json' -d "{\"long_url\":\"$LONG\",\"custom_alias\":\"$ALIAS\"}")
check "custom alias create" "201" "$ALIAS_STATUS"

echo "==> 5. alias conflict"
CONFLICT_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/api/v1/urls" \
  -H 'Content-Type: application/json' -d "{\"long_url\":\"$LONG\",\"custom_alias\":\"$ALIAS\"}")
check "alias conflict" "409" "$CONFLICT_STATUS"

echo "==> 6. validation: invalid URL"
BADURL_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/api/v1/urls" \
  -H 'Content-Type: application/json' -d '{"long_url":"not-a-url"}')
check "invalid url rejected" "400" "$BADURL_STATUS"

echo "==> 7. validation: past expiry"
PAST_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/api/v1/urls" \
  -H 'Content-Type: application/json' -d '{"long_url":"https://example.com","expires_at":"2000-01-01T00:00:00Z"}')
check "past expiry rejected" "400" "$PAST_STATUS"

echo "==> 8. unknown code"
UNKNOWN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/nonexistent-$RANDOM")
check "unknown code" "404" "$UNKNOWN_STATUS"

echo "==> 9. expiry: active then 410 Gone"
EXP=$(date -u -d "+3 seconds" +%Y-%m-%dT%H:%M:%SZ)
EXP_BODY=$(curl -s -X POST "$GATEWAY/api/v1/urls" -H 'Content-Type: application/json' \
  -d "{\"long_url\":\"$LONG\",\"expires_at\":\"$EXP\"}")
EXP_CODE=$(echo "$EXP_BODY" | json_field code)
ACTIVE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/$EXP_CODE")
check "expiring link active" "302" "$ACTIVE_STATUS"
echo "    waiting for expiry..."
sleep 4
EXPIRED_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/$EXP_CODE")
check "expired link returns Gone" "410" "$EXPIRED_STATUS"

echo "==> 10. rate limiting"
COUNT_429=$(seq 1 300 | xargs -P 30 -I {} curl -s -o /dev/null -w '%{http_code}\n' \
  "$GATEWAY/ratelimit-probe" | grep -c 429)
if [[ "$COUNT_429" -gt 0 ]]; then
  green "PASS: rate limiting triggered ($COUNT_429 requests got 429)"
  PASS=$((PASS + 1))
else
  red "FAIL: expected some 429 responses, got none"
  FAIL=$((FAIL + 1))
fi

echo
echo "================ e2e summary ================"
green "passed: $PASS"
[[ "$FAIL" -gt 0 ]] && red "failed: $FAIL"
echo "============================================="
[[ "$FAIL" -eq 0 ]]
