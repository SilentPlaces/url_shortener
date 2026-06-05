#!/usr/bin/env sh
set -eu

BASE_SHORTENER="${SHORTENER_API_BASE:-http://localhost:8080}"
BASE_REDIRECTOR="${REDIRECTOR_API_BASE:-http://localhost:8081}"

echo "Creating short URL..."
RESPONSE="$(curl -sS -X POST "$BASE_SHORTENER/api/v1/urls" \
  -H "Content-Type: application/json" \
  -d '{"original_url":"https://example.com/docs"}')"
echo "$RESPONSE"

if command -v jq >/dev/null 2>&1; then
  ALIAS="$(echo "$RESPONSE" | jq -r .alias)"
else
  ALIAS="$(echo "$RESPONSE" | sed -n 's/.*"alias":"\([^"]*\)".*/\1/p')"
fi
if [ -z "$ALIAS" ] || [ "$ALIAS" = "null" ]; then
  echo "Failed to parse alias from response"
  exit 1
fi

echo "Resolving redirect for alias: $ALIAS"
STATUS="$(curl -o /dev/null -s -w '%{http_code}' "$BASE_REDIRECTOR/$ALIAS")"
echo "HTTP status: $STATUS"

if [ "$STATUS" != "307" ] && [ "$STATUS" != "302" ] && [ "$STATUS" != "301" ]; then
  echo "Expected a redirect status, got $STATUS"
  exit 1
fi

echo "Smoke test completed."
