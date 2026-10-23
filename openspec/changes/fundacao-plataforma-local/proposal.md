# Proposal: Fundação da Plataforma Local

## Why

O PRD define um marketplace com backend Go, frontend Next.js e Kubernetes local, mas o repositório ainda não tem cluster, esqueleto de aplicação, pipeline nem observabilidade. Sem uma fundação reproduzível, cada fatia seguinte (identidade, catálogo, checkout) inventaria seu próprio ambiente, contratos e forma de deploy, tornando impossível validar os critérios de sucesso do MVP local.

Esta change cria a linha de base mínima para que as demais fatias possam ser desenvolvidas, testadas e promovidas sem retrabalho estrutural.

## What Changes

- Criar o cluster Kubernetes local dedicado `mercado-dev` com `kind`, ingress-nginx nas portas `8080`/`8443`, `local-path` como StorageClass e os namespaces `mercado`, `mercado-data` e `mercado-observability`.
- Aplicar `ResourceQuota` e `LimitRange` para caber no orçamento local e evitar que um componente consuma a RAM dos demais.
- Estabelecer o monorepo com `backend/` (API e worker Go), `frontend/` (app Next.js do comprador), `deploy/` (cluster, Kustomize base e overlay local), `scripts/` e `docs/adr/`.
- Subir a dependência de dados mínima: PostgreSQL e Redis. Meilisearch, MinIO, Redpanda e Mailpit entram nas changes que os consomem, não aqui.
- Adicionar middleware de runtime: `/healthz` e `/readyz` distintos, graceful shutdown, correlation ID, logs estruturados, rate limiting por IP/usuário/API key e recuperação de panic.
- Adicionar observabilidade mínima: métricas Prometheus, logs JSON, traces OpenTelemetry e dashboards básicos no Grafana.
- Adicionar migrations com estratégia expand/contract, rollback documentado e um harness de seed determinístico, ainda sem dados de catálogo.
- Publicar o contrato OpenAPI 3.1 inicial da API e validá-lo no CI.
- Criar os comandos `make dev-up`, `make dev-down`, `make seed`, `make test` e `make load`, além do `Tiltfile` para o inner loop.
- Inicializar o repositório Git com `.gitignore` para segredos e adicionar o arquivo `LICENSE` MIT previsto no PRD.
- Criar o pipeline de CI: em pull request apenas valida (lint, testes, build, OpenAPI lint, scan de dependências e de container); no merge para `main` apenas cria a tag de release, seguindo a regra de CI do projeto.

Fora do escopo desta change: autenticação, catálogo, busca, carrinho, checkout, pagamento, pedidos, notificações de produto, painéis de vendedor/admin e qualquer ambiente de produção.

## Capabilities

### New Capabilities

- `platform/local-environment`: cluster kind, ingress, namespaces, quotas, hostnames e ciclo de vida reproduzível do ambiente local.
- `platform/delivery`: pipeline de validação em PR, criação de tag no merge, build de imagens e validação do contrato OpenAPI.
- `platform/data`: migrations expand/contract e seed determinístico executáveis do zero.
- `platform/observability`: métricas, logs estruturados, traces e correlação entre eles.
- `platform/runtime`: health/readiness, graceful shutdown e rate limiting como comportamento de runtime da API.
- `platform/api-contract`: versionamento e publicação do contrato OpenAPI 3.1 da API.

### Modified Capabilities

Nenhuma. O projeto ainda não possui specs.

## Impact

- **Repositório:** cria a estrutura `backend/`, `frontend/`, `deploy/`, `scripts/` e `docs/adr/`, além de `Makefile` e `Tiltfile`.
- **Infraestrutura local:** novo cluster `mercado-dev`; não reutiliza `raiz-comum` nem `rancher`; portas `8080`/`8443` para não conflitar com o Umbrel.
- **Dependências:** Go 1.22+, Node.js, Next.js, PostgreSQL, Redis, ingress-nginx, Prometheus, Grafana, Jaeger, kind, Helm, kubectl, Tilt e k6.
- **CI:** novos workflows de pull request e de release na `main`.
- **Consumidores:** todas as changes seguintes (identidade, catálogo, carrinho, checkout, seller center, admin) dependem desta fundação.
- **Fora de produção:** nenhum manifesto desta change é promovido para produção; os overlays de staging/produção ficam para a change `prontidao-producao`.
