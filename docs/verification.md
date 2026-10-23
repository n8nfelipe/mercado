# Verificação integrada

Execução local em 2026-10-22, cluster kind `mercado-dev`, após remover e recriar o cluster.

## 11.1 Ambiente do zero em menos de 10 minutos

```bash
make dev-down
time make dev-up
```

O `make dev-down` removeu o cluster kind. O `make dev-up` criou cluster, ingress, data plane, observabilidade, imagens e workloads, aplicou migrations e terminou com:

```text
Mercado local ready:
  web:  http://mercado.localhost:8080
  api:  http://api.mercado.localhost:8080
  obs:  http://grafana.mercado.localhost:8080 / http://jaeger.mercado.localhost:8080
```

Tempo total medido: **435 segundos (7 min 15 s)**, abaixo do limite de 10 minutos.

Observação: o cache local de imagens Docker foi preservado entre as execuções. O cluster, os volumes e os workloads foram recriados do zero; um host sem cache pode ter tempo maior por causa do pull de imagens.

## 11.2 Testes e smoke

```bash
make test
make smoke
```

Resultado: `go test ./...` passou, o typecheck dos três pacotes do workspace passou e o smoke retornou:

```text
smoke ok: api=http://api.mercado.localhost:8080 web=http://mercado.localhost:8080
```

## 11.3 Endpoints, UI, tracing, dashboard e correlação

Verificações executadas:

- `GET /healthz` -> `200`.
- `GET /readyz` -> `200`.
- `GET /metrics` -> `200`, com métricas `http_requests_total` e `rate_limit_fail_open_total`.
- `GET /v1/status` -> `200` com `{"service":"mercado-api","version":"local","commit":"local","status":"ok"}`.
- Página do comprador `http://mercado.localhost:8080/` -> `200`, contendo o marcador `Mercado`.
- Jaeger: serviço `mercado-api` presente e trace `http.server` consultável.
- Grafana: health `database: ok` e dashboard `Mercado / RED e saturação` disponível.
- Correlação: `X-Correlation-ID: verify-clean-123` apareceu na resposta e no log da API com o mesmo valor.

## 11.4 Quotas, rate limit, encerramento e Redis indisponível

### Quotas e limites

Uma tentativa de criar um pod com `limits.cpu: 10` foi rejeitada:

```text
Error from server (Forbidden): pods "quota-check" is forbidden: maximum cpu usage per Container is 2, but limit is 10
```

A `ResourceQuota` do namespace `mercado` também foi inspecionada: 3/20 pods, 250m/2 CPU de requests, 320Mi/3Gi de memória e 2/10 services no momento da verificação.

### Rate limit

Foram feitas 140 requisições para `/v1/status`: 120 retornaram `200` e 20 retornaram `429`. A resposta `429` incluiu:

```text
Retry-After: 58
X-Correlation-Id: ... 
{"error":{"code":"rate_limited","message":"rate limit exceeded"},"correlation_id":"..."}
```

### Encerramento gracioso

`SIGTERM` foi enviado ao PID 1 da API. O container reiniciou (`restartCount` passou de 0 para 1), voltou a ficar `Ready` e o log anterior registrou:

```text
{"level":"INFO","msg":"API stopped"}
```

### Readiness com Redis indisponível

O StatefulSet do Redis foi reduzido para 0 réplicas. Resultados durante a indisponibilidade:

- `GET /readyz` -> `503`.
- `GET /v1/status` -> `200`, confirmando fail-open do rate limit.
- `rate_limit_fail_open_total` -> `1`.

Depois do Redis voltar a 1 réplica, a API voltou a ficar `Ready`.

## 11.5 Reproducibilidade e persistência

O `make dev-down` apaga o cluster kind e, portanto, os dados locais; o novo `make dev-up` recriou tudo. As migrations foram aplicadas novamente:

```text
OK 00001_init_reference_data.sql
OK 00002_add_locale_extra.sql
goose: successfully migrated database to version: 2
schema_version=2
```

Os PVCs de PostgreSQL e Redis foram recriados com StorageClass `local-path`. Reiniciar pods preserva os dados; remover o cluster não preserva. O comportamento está documentado em `docs/local-data.md`.

## 11.6 OpenSpec

```bash
openspec validate fundacao-plataforma-local --strict
openspec status --change fundacao-plataforma-local --json
```

Resultado: `Change 'fundacao-plataforma-local' is valid`, com artefatos de proposal, specs, design e tasks presentes.

## 11.7 Varredura de segredos

```bash
gitleaks detect --source . --no-git --redact --verbose
```

Resultado: `no leaks found`. O `.gitleaks.toml` permite apenas artefatos gerados e não versionados (`.next/`, `node_modules/` e `.pnpm-store/`); nenhum segredo, `.env.local` ou chave privada foi encontrado no conteúdo do projeto.
