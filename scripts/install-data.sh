#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

log() { printf '\n==> %s\n' "$*"; }

log "installing CloudNativePG operator"
helm repo add cnpg https://cloudnative-pg.github.io/charts >/dev/null 2>&1 || true
helm repo update cnpg >/dev/null
helm upgrade --install cnpg cnpg/cloudnative-pg \
  --namespace cnpg-system --create-namespace \
  --wait --timeout 300s

log "applying PostgreSQL cluster"
kubectl apply -f "${ROOT_DIR}/deploy/k8s/base/postgres-cluster.yaml"
kubectl wait --for=condition=Ready clusters.postgresql.cnpg.io/postgres -n mercado-data --timeout=300s

log "creating local application secrets"
"${ROOT_DIR}/scripts/dev-secrets.sh"

log "applying Redis"
kubectl apply -f "${ROOT_DIR}/deploy/k8s/base/redis.yaml"
kubectl rollout status statefulset/redis -n mercado-data --timeout=180s

log "applying MinIO"
kubectl apply -k "${ROOT_DIR}/deploy/k8s/data"
kubectl rollout status deployment/minio -n mercado-data --timeout=180s

log "creating MinIO bucket"
kubectl delete job minio-init -n mercado-data --ignore-not-found >/dev/null
kubectl apply -f "${ROOT_DIR}/deploy/k8s/base/jobs/minio-init.yaml"
kubectl wait --for=condition=complete job/minio-init -n mercado-data --timeout=120s
