#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

log() { printf '\n==> %s\n' "$*"; }

log "validating OpenAPI"
pnpm dlx --package=@redocly/cli@1.34.2 redocly lint api/openapi.yaml

BASE_FILE="${API_BASE_FILE:-}"
CREATED_BASE=0
if [[ -z "${BASE_FILE}" ]]; then
  BASE_REF="${API_BASE_REF:-main}"
  if git rev-parse --verify --quiet "${BASE_REF}" >/dev/null; then
    BASE_FILE="$(mktemp)"
    CREATED_BASE=1
    git show "${BASE_REF}:api/openapi.yaml" > "${BASE_FILE}"
  elif git rev-parse --verify --quiet "origin/${BASE_REF}" >/dev/null; then
    BASE_FILE="$(mktemp)"
    CREATED_BASE=1
    git show "origin/${BASE_REF}:api/openapi.yaml" > "${BASE_FILE}"
  else
    log "base ref ${BASE_REF} not found; skipping oasdiff"
    exit 0
  fi
fi

BASE_FILE="$(realpath "${BASE_FILE}")"
REVISION_FILE="${API_REVISION_FILE:-${ROOT_DIR}/api/openapi.yaml}"

cleanup() {
  if [[ "${CREATED_BASE}" -eq 1 ]]; then
    rm -f "${BASE_FILE}"
  fi
}
trap cleanup EXIT

log "checking breaking changes against ${BASE_FILE}"
cd "${ROOT_DIR}/backend"
GOTOOLCHAIN=local go run github.com/tufin/oasdiff@v1.10.0 breaking \
  "${BASE_FILE}" "${REVISION_FILE}" --fail-on ERR
