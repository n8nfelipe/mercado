#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEED="${SEED:-1}"
DB_PORT="${SEED_DB_PORT:-15432}"
PF_PID=""

log() { printf '\n==> %s\n' "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
cleanup() {
  if [[ -n "${PF_PID}" ]]; then
    kill "${PF_PID}" 2>/dev/null || true
    wait "${PF_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

if [[ -z "${DATABASE_URL:-}" ]]; then
  command -v kubectl >/dev/null 2>&1 || fail "kubectl is required when DATABASE_URL is not set"
  kubectl get secret postgres-app -n mercado-data >/dev/null 2>&1 || \
    fail "postgres-app secret not found; run make dev-up first"

  log "starting PostgreSQL port-forward on 127.0.0.1:${DB_PORT}"
  kubectl port-forward -n mercado-data svc/postgres-rw "${DB_PORT}:5432" \
    >/tmp/mercado-seed-port-forward.log 2>&1 &
  PF_PID=$!

  for i in $(seq 1 30); do
    if (echo >"/dev/tcp/127.0.0.1/${DB_PORT}") >/dev/null 2>&1; then
      break
    fi
    if ! kill -0 "${PF_PID}" 2>/dev/null; then
      cat /tmp/mercado-seed-port-forward.log >&2 || true
      fail "port-forward exited before the database was reachable"
    fi
    if [[ "${i}" -eq 30 ]]; then
      fail "database did not become reachable on port ${DB_PORT}"
    fi
    sleep 1
  done

  DB_URI="$(kubectl get secret postgres-app -n mercado-data -o jsonpath='{.data.uri}' | base64 -d)"
  [[ -n "${DB_URI}" ]] || fail "database URI is empty"
  DATABASE_URL="$(printf '%s' "${DB_URI}" | sed "s#@postgres-rw.mercado-data:5432#@127.0.0.1:${DB_PORT}#")"
  if [[ "${DATABASE_URL}" != *"sslmode="* ]]; then
    if [[ "${DATABASE_URL}" == *"?"* ]]; then
      DATABASE_URL="${DATABASE_URL}&sslmode=disable"
    else
      DATABASE_URL="${DATABASE_URL}?sslmode=disable"
    fi
  fi
  export DATABASE_URL
fi

cd "${ROOT_DIR}/backend"
log "applying migrations"
go run ./cmd/migrate -direction up
if [[ -n "${SEED_SELLER_EMAIL:-}" ]]; then
  [[ -n "${SEED_SELLER_PASSWORD:-}" ]] || fail "SEED_SELLER_PASSWORD is required when SEED_SELLER_EMAIL is set"
  log "seeding catalog data (seller ${SEED_SELLER_EMAIL}, listings published)"
else
  log "seeding reference data only (set SEED_SELLER_EMAIL/SEED_SELLER_PASSWORD to create sample listings)"
fi
log "running seed ${SEED}"
go run ./cmd/seed -seed "${SEED}"
