#!/usr/bin/env bash
set -euo pipefail

WEB_URL="${WEB_URL:-http://mercado.localhost:8080}"
API_URL="${API_URL:-http://api.mercado.localhost:8080}"

fail() { printf 'smoke: %s\n' "$*" >&2; exit 1; }

wait_for() {
  local url="$1"
  for _ in $(seq 1 30); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  fail "timeout waiting for ${url}"
}

wait_for "${API_URL}/healthz"
wait_for "${API_URL}/readyz"

status="$(curl -fsS "${API_URL}/v1/status")"
printf '%s' "${status}" | grep -q '"status"' || fail "unexpected /v1/status response: ${status}"

page="$(curl -fsS "${WEB_URL}/")"
printf '%s' "${page}" | grep -q 'Mercado' || fail "web page missing Mercado marker"

for host in seller.mercado.localhost admin.mercado.localhost; do
  code="$(curl -sS -o /dev/null -w '%{http_code}' -H "Host: ${host}" "${WEB_URL}/")"
  [[ "${code}" == "404" ]] || fail "${host} expected 404, got ${code}"
done

printf 'smoke ok: api=%s web=%s\n' "${API_URL}" "${WEB_URL}"
