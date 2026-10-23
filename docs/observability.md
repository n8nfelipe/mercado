# Observabilidade local

## Componentes

| Componente | Implementação | Retenção |
|---|---|---|
| Métricas | Prometheus + node-exporter + kube-state-metrics | 24 h em `emptyDir` |
| Dashboards | Grafana 12 | provisionado por ConfigMap |
| Traces | Jaeger all-in-one | em memória, 10.000 traces |

Alertmanager e armazenamento de longo prazo não fazem parte do ambiente local.

## URLs

- Grafana: `http://grafana.mercado.localhost:8080`
- Jaeger: `http://jaeger.mercado.localhost:8080`
- Prometheus: interno em `http://prometheus-server.mercado-observability.svc.cluster.local:80`; para acessar do host, use `kubectl port-forward`.

```bash
kubectl port-forward -n mercado-observability svc/prometheus-server 9090:80
# http://127.0.0.1:9090
```

## Credenciais

O usuário do Grafana é `admin`. A senha é gerada localmente e fica no secret `grafana-admin`:

```bash
kubectl get secret grafana-admin -n mercado-observability \
  -o jsonpath='{.data.admin-password}' | base64 -d; echo
```

O secret não é versionado. Para rotacionar, remova `grafana-admin`, rode `./scripts/dev-secrets.sh` e reinicie o Grafana.

## Provisionamento

- `deploy/k8s/observability/grafana-datasources.yaml`: datasources Prometheus e Jaeger.
- `deploy/k8s/observability/grafana-dashboards.yaml`: dashboard `Mercado / RED e saturação`.
- `deploy/k8s/observability/grafana-values.yaml`: provider de dashboards, montagem dos ConfigMaps e recursos.
- `deploy/k8s/observability/prometheus-values.yaml`: retenção, recursos e scrape de node-exporter, kube-state-metrics e da API (`/metrics`, a cada 10s).
- `deploy/k8s/observability/jaeger.yaml`: Jaeger all-in-one com OTLP.
- `deploy/k8s/observability/ingress.yaml`: hosts do Grafana e do Jaeger.

O dashboard e as datasources são restaurados ao recriar o pod do Grafana, sem configuração manual.

## OTLP

A API deve exportar traces para:

- gRPC: `jaeger.mercado-observability.svc.cluster.local:4317`
- HTTP: `http://jaeger.mercado-observability.svc.cluster.local:4318`

Exemplo de span sintético via OTLP HTTP:

```bash
kubectl port-forward -n mercado-observability svc/jaeger 4318:4318 16686:16686
TRACE_ID="$(openssl rand -hex 16)"
SPAN_ID="$(openssl rand -hex 8)"
NOW="$(date +%s%N)"
END="$((NOW + 1000000))"
cat > /tmp/span.json <<EOF
{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"smoke-test"}}]},"scopeSpans":[{"scope":{"name":"smoke"},"spans":[{"traceId":"${TRACE_ID}","spanId":"${SPAN_ID}","name":"synthetic-span","kind":1,"startTimeUnixNano":"${NOW}","endTimeUnixNano":"${END}","status":{"code":1}}]}]}]}
EOF
curl -sS -X POST http://127.0.0.1:4318/v1/traces \
  -H 'Content-Type: application/json' -d @/tmp/span.json
curl -sS 'http://127.0.0.1:16686/api/services' | jq .
```

## Limitações

- Métricas e traces são locais e efêmeros; reiniciar o Prometheus ou o Jaeger perde o histórico.
- Sem Alertmanager, sem gravação remota e sem HA.
- O Grafana baixa plugins na primeira inicialização; aguarde o pod ficar `Ready`.
- As métricas RED da API são coletadas de `/metrics` e ficam disponíveis após o primeiro scrape (10s).
