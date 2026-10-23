#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_SECRET_NAMESPACE="${DB_SECRET_NAMESPACE:-mercado-data}"
DB_SECRET_NAME="${DB_SECRET_NAME:-postgres-app}"
SECRET_NAME="${SECRET_NAME:-mercado-secrets}"
SECRET_NAMESPACES="${SECRET_NAMESPACES:-mercado mercado-data}"

log() { printf '\n==> %s\n' "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

for i in $(seq 1 60); do
  if kubectl get secret "${DB_SECRET_NAME}" -n "${DB_SECRET_NAMESPACE}" >/dev/null 2>&1; then
    break
  fi
  sleep 5
  if [[ "$i" -eq 60 ]]; then
    fail "secret ${DB_SECRET_NAMESPACE}/${DB_SECRET_NAME} did not appear"
  fi
done

DB_URI="$(kubectl get secret "${DB_SECRET_NAME}" -n "${DB_SECRET_NAMESPACE}" -o jsonpath='{.data.uri}' | base64 -d)"
[[ -n "${DB_URI}" ]] || fail "database uri is empty"

REDIS_PASSWORD="$(kubectl get secret "${SECRET_NAME}" -n "${DB_SECRET_NAMESPACE}" -o jsonpath='{.data.redis-password}' 2>/dev/null | base64 -d || true)"
if [[ -z "${REDIS_PASSWORD}" ]]; then
  REDIS_PASSWORD="$(openssl rand -base64 24 | tr -d '\n')"
  log "generated local Redis password"
fi

MINIO_ACCESS_KEY="$(kubectl get secret "${SECRET_NAME}" -n "${DB_SECRET_NAMESPACE}" -o jsonpath='{.data.minio-access-key}' 2>/dev/null | base64 -d || true)"
if [[ -z "${MINIO_ACCESS_KEY}" ]]; then
  MINIO_ACCESS_KEY="$(openssl rand -hex 16)"
  log "generated local MinIO access key"
fi
MINIO_SECRET_KEY="$(kubectl get secret "${SECRET_NAME}" -n "${DB_SECRET_NAMESPACE}" -o jsonpath='{.data.minio-secret-key}' 2>/dev/null | base64 -d || true)"
if [[ -z "${MINIO_SECRET_KEY}" ]]; then
  MINIO_SECRET_KEY="$(openssl rand -hex 32)"
  log "generated local MinIO secret key"
fi

for namespace in ${SECRET_NAMESPACES}; do
  kubectl create secret generic "${SECRET_NAME}" \
    --namespace "${namespace}" \
    --from-literal=database-url="${DB_URI}" \
    --from-literal=redis-password="${REDIS_PASSWORD}" \
    --from-literal=minio-access-key="${MINIO_ACCESS_KEY}" \
    --from-literal=minio-secret-key="${MINIO_SECRET_KEY}" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  echo "applied secret ${namespace}/${SECRET_NAME}"
done

GRAFANA_NAMESPACE="${GRAFANA_NAMESPACE:-mercado-observability}"
GRAFANA_SECRET="${GRAFANA_SECRET:-grafana-admin}"
GRAFANA_PASSWORD="$(kubectl get secret "${GRAFANA_SECRET}" -n "${GRAFANA_NAMESPACE}" -o jsonpath='{.data.admin-password}' 2>/dev/null | base64 -d || true)"
if [[ -z "${GRAFANA_PASSWORD}" ]]; then
  GRAFANA_PASSWORD="$(openssl rand -base64 24 | tr -d '\n')"
  log "generated local Grafana admin password"
fi
kubectl create secret generic "${GRAFANA_SECRET}" \
  --namespace "${GRAFANA_NAMESPACE}" \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="${GRAFANA_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
echo "applied secret ${GRAFANA_NAMESPACE}/${GRAFANA_SECRET}"
