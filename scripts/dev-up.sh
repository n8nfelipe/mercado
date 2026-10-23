#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-mercado-dev}"
CONTEXT="kind-${CLUSTER_NAME}"

log() { printf '\n==> %s\n' "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

preflight_ports() {
  if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
    return 0
  fi
  for port in 8080 8443; do
    if ss -ltnH "sport = :${port}" 2>/dev/null | grep -q .; then
      fail "host port ${port} is already in use; stop the process using it or set a different mapping"
    fi
  done
}

ensure_cluster() {
  if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
    log "cluster ${CLUSTER_NAME} already exists"
  else
    log "creating kind cluster ${CLUSTER_NAME}"
    kind create cluster --name "${CLUSTER_NAME}" --config "${ROOT_DIR}/deploy/kind/cluster.yaml" --wait 120s
  fi
  kubectl config use-context "${CONTEXT}" >/dev/null
}

ensure_ingress() {
  log "installing ingress-nginx"
  helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx >/dev/null 2>&1 || true
  helm repo update ingress-nginx >/dev/null
  helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
    --namespace ingress-nginx --create-namespace \
    --set controller.kind=DaemonSet \
    --set controller.service.type=NodePort \
    --set controller.hostPort.enabled=true \
    --set-string controller.nodeSelector.ingress-ready=true \
    --set controller.tolerations[0].key=node-role.kubernetes.io/control-plane \
    --set controller.tolerations[0].operator=Exists \
    --set controller.tolerations[0].effect=NoSchedule \
    --set controller.admissionWebhooks.enabled=false \
    --set controller.watchIngressWithoutClass=true \
    --wait --timeout 300s
}

ensure_governance() {
  log "applying storage class, namespaces, quotas and limits"
  kubectl apply -f "${ROOT_DIR}/deploy/k8s/base/storageclass.yaml"
  kubectl apply -f "${ROOT_DIR}/deploy/k8s/base/namespaces.yaml"
  kubectl delete validatingadmissionpolicy mercado-require-resources --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete validatingadmissionpolicybinding mercado-require-resources --ignore-not-found >/dev/null 2>&1 || true
}

install_data() {
  if [[ -f "${ROOT_DIR}/deploy/k8s/base/postgres-cluster.yaml" ]]; then
    log "installing local data plane"
    "${ROOT_DIR}/scripts/install-data.sh"
  fi
}

install_observability() {
  if [[ -f "${ROOT_DIR}/scripts/install-observability.sh" ]]; then
    log "installing observability"
    "${ROOT_DIR}/scripts/install-observability.sh"
  fi
}

build_and_deploy() {
  if [[ -f "${ROOT_DIR}/backend/go.mod" ]]; then
    log "building and loading application images"
    "${ROOT_DIR}/scripts/build-images.sh"
  fi
  if [[ -f "${ROOT_DIR}/deploy/k8s/overlays/local/kustomization.yaml" ]]; then
    log "deploying application"
    kubectl apply -k "${ROOT_DIR}/deploy/k8s/overlays/local"
    kubectl rollout status deployment/api -n mercado --timeout=180s
    kubectl rollout status deployment/web -n mercado --timeout=180s
    kubectl rollout status deployment/worker -n mercado --timeout=120s
  fi
  if [[ -f "${ROOT_DIR}/deploy/k8s/base/jobs/migrate.yaml" ]]; then
    log "running migrations"
    kubectl delete job migrate -n mercado --ignore-not-found
    kubectl apply -f "${ROOT_DIR}/deploy/k8s/base/jobs/migrate.yaml"
    kubectl wait --for=condition=complete job/migrate -n mercado --timeout=180s
  fi
}

print_urls() {
  printf '\nMercado local ready:\n'
  printf '  web:  http://mercado.localhost:8080\n'
  printf '  api:  http://api.mercado.localhost:8080\n'
  printf '  obs:  http://grafana.mercado.localhost:8080 / http://jaeger.mercado.localhost:8080\n'
}

require kind
require kubectl
require helm
require docker
require ss

preflight_ports
ensure_cluster
ensure_ingress
ensure_governance
install_data
install_observability
build_and_deploy
print_urls
