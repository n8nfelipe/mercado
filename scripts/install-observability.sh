#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OBS_DIR="${ROOT_DIR}/deploy/k8s/observability"
NAMESPACE="mercado-observability"

log() { printf '\n==> %s\n' "$*"; }

log "creating observability credentials"
"${ROOT_DIR}/scripts/dev-secrets.sh"

log "installing Prometheus, node-exporter and kube-state-metrics"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo update prometheus-community >/dev/null
helm upgrade --install prometheus prometheus-community/prometheus \
  --namespace "${NAMESPACE}" \
  --values "${OBS_DIR}/prometheus-values.yaml" \
  --wait --timeout 600s

log "applying Jaeger and Grafana provisioning"
kubectl apply -f "${OBS_DIR}/grafana-datasources.yaml"
kubectl apply -f "${OBS_DIR}/grafana-dashboards.yaml"
kubectl apply -f "${OBS_DIR}/jaeger.yaml"
kubectl rollout status deployment/jaeger -n "${NAMESPACE}" --timeout=180s

log "installing Grafana"
helm repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
helm repo update grafana >/dev/null
helm upgrade --install grafana grafana/grafana \
  --namespace "${NAMESPACE}" \
  --values "${OBS_DIR}/grafana-values.yaml" \
  --wait --timeout 300s

log "applying observability ingress"
kubectl apply -f "${OBS_DIR}/ingress.yaml"
