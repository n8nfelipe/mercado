# Runtime da API

## Visão geral

- Go 1.22, `net/http` + `chi`, `pgx/v5` e `go-redis/v9`.
- Logs em JSON com `slog`.
- Métricas Prometheus em `/metrics`.
- Traces OpenTelemetry exportados por OTLP gRPC.
- Binários: `cmd/api` e `cmd/worker`.

## Configuração

A API falha na inicialização quando uma variável obrigatória está ausente ou inválida.

| Variável | Obrigatória | Padrão | Uso |
|---|---|---|---|
| `DATABASE_URL` | sim | - | DSN do PostgreSQL |
| `REDIS_ADDR` | sim | - | host:porta do Redis |
| `ADDR` | não | `:PORT` | endereço de escuta |
| `PORT` | não | `8080` | porta usada quando `ADDR` não é definido |
| `REDIS_PASSWORD` | não | vazio | senha do Redis |
| `REDIS_DB` | não | `0` | database do Redis |
| `SERVICE_NAME` | não | `mercado-api` | nome em logs e traces |
| `SERVICE_VERSION` | não | `dev` | versão em `/v1/status` e traces |
| `GIT_COMMIT` | não | `none` | commit em `/v1/status` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | não | vazio | URL do coletor OTLP, como `http://jaeger.mercado-observability.svc.cluster.local:4317` |
| `LOG_LEVEL` | não | `info` | `debug`, `info`, `warn` ou `error` |
| `SHUTDOWN_TIMEOUT` | não | `10s` | drenagem no `SIGTERM` |
| `READ_TIMEOUT` | não | `10s` | leitura da requisição |
| `WRITE_TIMEOUT` | não | `15s` | escrita da resposta |
| `IDLE_TIMEOUT` | não | `60s` | conexões ociosas |
| `READ_HEADER_TIMEOUT` | não | `5s` | cabeçalhos |
| `READINESS_TIMEOUT` | não | `2s` | ping de dependências |
| `MAX_BODY_BYTES` | não | `1048576` | tamanho máximo do corpo |
| `RATE_LIMIT_ENABLED` | não | `true` | liga o limitador |
| `RATE_LIMIT_IP_PER_MINUTE` | não | `120` | limite por IP |
| `RATE_LIMIT_API_KEY_PER_MINUTE` | não | `600` | limite por API key |
| `RATE_LIMIT_WINDOW` | não | `1m` | janela do limitador |
| `TRUST_PROXY` | não | `true` | confia em `X-Forwarded-For`/`X-Real-IP` |
| `SESSION_COOKIE_NAME` | não | `mercado_session` | nome do cookie de sessão |
| `SESSION_TTL` | não | `168h` | validade absoluta da sessão |
| `SESSION_SECURE` | não | `false` | força `Secure` no cookie; HTTPS também ativa automaticamente |
| `ALLOWED_ORIGINS` | não | URLs locais do web | origens aceitas em requisições mutantes |
| `LOGIN_RATE_LIMIT_PER_MINUTE` | não | `10` | tentativas de login por conta+IP |
| `MINIO_ENDPOINT` | não | `minio.mercado-data.svc.cluster.local:9000` | host:porta da API S3 do MinIO |
| `MINIO_ACCESS_KEY` | não | vazio | access key; vazio desativa o storage de imagens |
| `MINIO_SECRET_KEY` | não | vazio | secret key; deve ser definida junto com a access key |
| `MINIO_BUCKET` | não | `mercado-listings` | bucket dos objetos de imagem |
| `MINIO_USE_SSL` | não | `false` | usa HTTPS na conexão com o MinIO |

`MINIO_ACCESS_KEY` e `MINIO_SECRET_KEY` devem ser definidas juntas. Sem credenciais, a API inicia com o storage desabilitado, o upload responde `503` e a mídia responde `404`. O bucket precisa existir antes da API subir; `scripts/install-data.sh` cria o bucket via job `minio-init`.

`backend/.env.example` contém um exemplo sem segredos.

## Sessão, cookies e CSRF

- O login emite um token opaco de 32 bytes; o banco guarda apenas o SHA-256 em `sessions.token_hash`.
- O cookie usa `HttpOnly`, `SameSite=Lax`, `Path=/` e `Secure` quando a requisição é HTTPS ou `SESSION_SECURE=true`.
- A sessão é absoluta (`SESSION_TTL`, padrão 7 dias) e é revogada no logout.
- Cada login cria um novo identificador de sessão; o anterior não é reutilizado.
- Requisições mutantes com `Origin` exigem uma origem em `ALLOWED_ORIGINS` ou igual ao host da API. Sem `Origin`, `Sec-Fetch-Site: cross-site` é rejeitado.
- O login limita tentativas por `hash(conta)+IP` no Redis. Ao exceder, responde `429` com `Retry-After` e mensagem genérica.
- Senhas, hashes e cookies não aparecem em logs, respostas ou métricas.

Fluxo manual contra a API local:

```bash
curl -i -c /tmp/mercado-cookies.txt http://127.0.0.1:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"senha-de-teste"}'
curl -sS -b /tmp/mercado-cookies.txt http://127.0.0.1:8080/v1/auth/me
curl -i -b /tmp/mercado-cookies.txt -X POST http://127.0.0.1:8080/v1/auth/logout
```

## Health e readiness

- `GET /healthz`: liveness. Responde `200` com `{"status":"ok"}` sem consultar dependências.
- `GET /readyz`: readiness. Consulta PostgreSQL e Redis com timeout e responde `200` quando ambos estão `up` ou `503` com o mapa `checks`.

```bash
curl -sS localhost:8080/healthz
curl -sS localhost:8080/readyz
```

## Encerramento gracioso

No `SIGTERM` ou `SIGINT`, o servidor para de aceitar conexões, conclui requisições em andamento dentro de `SHUTDOWN_TIMEOUT` e encerra. Se o timeout expirar, o servidor é fechado e a falha é registrada no log.

## Logs e correlação

Cada requisição emite um JSON com serviço, correlação, método, rota, status, latência, IP, bytes e trace ID. O cabeçalho `X-Correlation-ID` é aceito quando válido ou gerado, e sempre devolvido na resposta. Valores de query sensíveis são mascarados; headers e corpo nunca são registrados.

Exemplo:

```json
{"msg":"http request","service":"mercado-api","correlation_id":"...","method":"GET","path":"/v1/status","route":"/v1/status","status":200,"duration_ms":1,"trace_id":"..."}
```

## Métricas

`GET /metrics` expõe:

- `http_requests_total{route,method,status}`
- `http_request_duration_seconds{route,method}`
- `rate_limit_fail_open_total`

Rotas sem tráfego não geram séries. O próprio `/metrics` não é contabilizado.

## Traces

Com `OTEL_EXPORTER_OTLP_ENDPOINT` definido, a API exporta spans HTTP, PostgreSQL e Redis. O trace ID aparece no log da requisição. No ambiente local, o destino é o Jaeger:

```bash
kubectl port-forward -n mercado-observability svc/jaeger 16686:16686
```

Abra `http://127.0.0.1:16686` e filtre por `mercado-api`.

## Rate limiting

O limitador usa Redis e uma janela fixa por chave. Aplica limite por IP e, quando `X-API-Key` está presente, também por API key (a chave é armazenada como hash). Ao exceder, responde `429` com `Retry-After` e envelope de erro. `/healthz`, `/readyz` e `/metrics` não são limitados.

Se o Redis estiver indisponível, a API libera a requisição (fail-open), incrementa `rate_limit_fail_open_total` e registra um aviso.

## Limites de requisição

Corpo acima de `MAX_BODY_BYTES` recebe `413`. `POST /v1/seller/listings/{listingId}/images` é a exceção: aceita até 5 MiB de arquivo mais o overhead do multipart, e cada arquivo acima de 5 MiB recebe `413`. Os timeouts de leitura, escrita, cabeçalhos e idle são aplicados pelo `http.Server`.

## Build e testes

```bash
cd backend
go build ./...
go test ./...
go vet ./...
```

## Imagem

```bash
docker build -t mercado-api:local --build-arg VERSION=local --build-arg COMMIT="$(git rev-parse --short HEAD)" backend
docker run --rm -p 18080:8080 \
  -e ADDR=0.0.0.0:8080 \
  -e DATABASE_URL=postgres://demo:demo@127.0.0.1:1/demo \
  -e REDIS_ADDR=127.0.0.1:1 \
  mercado-api:local
curl -sS http://127.0.0.1:18080/healthz
```

A imagem usa usuário não-root `mercado` e healthcheck em `/healthz`.

## Execução contra o cluster local

```bash
kubectl port-forward -n mercado-data svc/postgres-rw 15432:5432
kubectl port-forward -n mercado-data svc/redis 16379:6379
kubectl port-forward -n mercado-data svc/minio 15434:9000
kubectl port-forward -n mercado-observability svc/jaeger 4317:4317

export DATABASE_URL="$(kubectl get secret postgres-app -n mercado-data -o jsonpath='{.data.uri}' | base64 -d | sed 's#@postgres-rw.mercado-data:5432#@127.0.0.1:15432#')?sslmode=disable"
export REDIS_ADDR=127.0.0.1:16379
export REDIS_PASSWORD="$(kubectl get secret mercado-secrets -n mercado-data -o jsonpath='{.data.redis-password}' | base64 -d)"
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
export MINIO_ENDPOINT=127.0.0.1:15434
export MINIO_ACCESS_KEY="$(kubectl get secret mercado-secrets -n mercado-data -o jsonpath='{.data.minio-access-key}' | base64 -d)"
export MINIO_SECRET_KEY="$(kubectl get secret mercado-secrets -n mercado-data -o jsonpath='{.data.minio-secret-key}' | base64 -d)"
export MINIO_BUCKET=mercado-listings
export ADDR=127.0.0.1:18080
go run ./cmd/api
```

## Degradação

- PostgreSQL fora: `/healthz` continua `200`; `/readyz` responde `503` com `postgres: down`.
- Redis fora: `/readyz` responde `503`; o rate limit falha aberto e registra a métrica.
- Jaeger fora: a aplicação continua atendendo; o exportador descarta ou tenta reenviar spans sem bloquear a requisição.
- MinIO fora: `/readyz` continua `200`; upload e leitura de mídia falham com `500`/`404`. Sem credenciais configuradas, o storage fica indisponível em vez de derrubar a API.
