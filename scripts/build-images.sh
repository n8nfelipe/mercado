#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-mercado-dev}"
TAG="${TAG:-local}"
VERSION="${VERSION:-local}"
COMMIT="${COMMIT:-local}"

log() { printf '\n==> %s\n' "$*"; }

log "building mercado-api:${TAG}"
docker build \
  --build-arg "VERSION=${VERSION}" \
  --build-arg "COMMIT=${COMMIT}" \
  -t "mercado-api:${TAG}" \
  "${ROOT_DIR}/backend"

docker tag "mercado-api:${TAG}" "mercado-worker:${TAG}"

log "building mercado-web:${TAG}"
docker build -t "mercado-web:${TAG}" -f "${ROOT_DIR}/frontend/apps/web/Dockerfile" "${ROOT_DIR}"

log "loading images into kind/${CLUSTER_NAME}"
kind load docker-image --name "${CLUSTER_NAME}" \
  "mercado-api:${TAG}" \
  "mercado-worker:${TAG}" \
  "mercado-web:${TAG}"
