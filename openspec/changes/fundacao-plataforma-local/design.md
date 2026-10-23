# Design: Fundação da Plataforma Local

## Context

O repositório contém apenas `PRD.md` e a estrutura OpenSpec; não há código, pipeline nem repositório Git inicializado. O host tem 8 vCPU, cerca de 9,5 GB de RAM disponível e ~32 GB de disco livre. As portas 80/443 pertencem ao Umbrel, e os clusters kind de outros projetos estão parados.

Esta é a primeira change de uma sequência. Identidade, catálogo, carrinho e checkout dependem do ambiente, dos contratos e da observabilidade criados aqui. Para não repetir o orçamento de RAM, Meilisearch, MinIO, Redpanda, Mailpit e Dex **não** entram nesta change: cada um é instalado pela change que o consome.

A regra de CI do projeto prevalece sobre o PRD: pull request roda somente verificações; merge na `main` roda somente a criação da tag de release. O PRD (seção 15.1) também prevê build, assinatura e push no merge, mas isso é adiado para a change de prontidão para produção.

## Goals / Non-Goals

**Goals:**

- Provisionar o cluster `mercado-dev` e o stack mínimo de C1 em menos de 10 minutos, do zero.
- Estabelecer o esqueleto Go/Next.js, os middlewares de runtime e os contratos que as próximas changes vão consumir.
- Entregar observabilidade correlacionada suficiente para diagnosticar uma requisição de ponta a ponta.
- Fazer migrations, seed, OpenAPI e CI serem executáveis e verificáveis desde o primeiro commit.
- Manter o consumo de C1 em torno de 4 GB de RAM, deixando margem para os stores das próximas changes.

**Non-Goals:**

- Autenticação, catálogo, busca, carrinho, checkout, pagamento, pedidos, notificações de produto, seller center e admin.
- Overlays de staging/produção, HA, multi-AZ, assinatura de imagem, publicação em registry e Argo CD local.
- Os três frontends do PRD; C1 entrega apenas o app do comprador, com `backoffice` reservado para C6/C7.
- Testes de carga completos; C1 apenas deixa o alvo `make load` e o esqueleto k6.

## Decisions

### 1. Cluster kind de um control-plane, portas 8080/8443

O cluster `mercado-dev` usa kind com um control-plane e workers adicionais apenas se houver RAM e disco liberados. O ingress-nginx usa o provider `kind` e publica as portas de host `8080` e `8443`, porque 80/443 estão ocupadas.

**Alternativas:** k3d (mais leve, mas menos alinhado ao PRD e ao ecossistema kind já instalado); minikube (mais recursos). kind foi escolhido por já estar instalado, ser descartável e mapear portas de forma explícita.

**Consequência:** o cluster não é HA por definição. Isso é uma limitação aceita, não um defeito a corrigir nesta fase.

### 2. Três namespaces com quota e limite

`mercado`, `mercado-data` e `mercado-observability` separam blast radius, cobrança de recursos e ciclo de vida. `ResourceQuota` e `LimitRange` são aplicados antes dos workloads, com defaults explícitos de requests/limits. Todo pod resultante tem requests/limits, e pods acima do máximo são rejeitados.

**Alternativas:** um único namespace (mais simples, mas mistura dados e aplicação e dificulta quota); um namespace por serviço (excessivo para o MVP).

### 3. CloudNativePG para PostgreSQL e Redis em StatefulSet

C1 instala PostgreSQL via CloudNativePG com uma instância e Redis como StatefulSet de um pod. Meilisearch, MinIO, Redpanda e Mailpit ficam para C3/C5.

**Alternativas:** PostgreSQL em StatefulSet puro (menos operador, menos RAM, mas perde backups e alinhamento com o stack do PRD); serviços gerenciados locais (não existem). CloudNativePG foi escolhido para manter o caminho de produção próximo, aceitando o custo do operador. Se o operador estourar o orçamento, o fallback é StatefulSet puro, registrado em ADR.

### 4. Observabilidade mínima com retenção curta

O stack é Prometheus + Grafana + Jaeger, com `kube-state-metrics` e `node-exporter`. A retenção local é de 24 h, sem Alertmanager e sem gravação de longo prazo. O objetivo é diagnóstico, não retenção analítica.

**Alternativas:** `kube-prometheus-stack` completo (inclui Alertmanager e regras que não são usadas agora, consumindo RAM); Tempo no lugar de Jaeger (mais alinhado à produção, mas mais um componente para operar no MVP). Jaeger all-in-one é suficiente para traces locais.

### 5. Backend Go com chi, pgx/sqlc, slog e OpenTelemetry

O backend segue o PRD: `net/http` + `chi`, `pgx/v5` + `sqlc`, `slog` para logs JSON, `prometheus/client_golang` para métricas RED e OpenTelemetry para traces. A configuração é lida de variáveis de ambiente e validada na inicialização.

**Alternativas:** `net/http` puro (menos dependências, mas roteamento e middlewares manuais); ORM (mais rápido no início, mas foge do `sqlc` e esconde SQL). A escolha mantém SQL explícito e instrumentação simples.

### 6. Migrations com goose e expand/contract

As migrations ficam em `backend/migrations` como SQL versionado e são embutidas no binário de migração com `goose`. Toda mudança segue expand/contract; remoções são migrations separadas. O CI aplica as migrations contra um PostgreSQL efêmero (Testcontainers) antes de aceitar o PR.

**Alternativas:** `golang-migrate` (equivalente, mas sem o mesmo suporte a embed); Atlas (mais poderoso, mas pesado para o MVP).

### 7. Seed determinístico com dados de referência

`backend/cmd/seed` usa uma semente explícita e upserts idempotentes. Em C1 ele popula apenas dados de referência de plataforma: países, moedas e locales (por exemplo, BR, BRL, pt-BR). As changes seguintes adicionam seeds de catálogo e pedidos sem alterar o contrato do comando.

**Alternativas:** fixtures SQL estáticas (simples, mas não reproduzem volumes nem variação); seed acoplado aos testes (não serve para o ambiente local).

### 8. Segredos locais gerados, nunca versionados

Variáveis de ambiente são validadas no boot. Segredos locais são gerados por `scripts/dev-secrets.sh` e aplicados como Kubernetes Secrets; `.env.local` e artefatos equivalentes ficam no `.gitignore`. O CI usa secrets do GitHub.

**Alternativas:** segredos de desenvolvimento commitados (rejeitado por segurança); Vault local (mais um componente e mais RAM).

### 9. Ordem de middlewares e rate limiting distribuído

A ordem externa para interna é: recuperação de panic, correlation ID, access log, métricas/tracing, rate limiting e router. `/healthz`, `/readyz` e `/metrics` ficam fora do rate limiting. O limitador é um token bucket em Redis via script Lua, com chave por IP ou API key; quando houver autenticação, também por usuário. Em falha do Redis, o limitador falha aberto e incrementa uma métrica de degradação.

**Alternativas:** limitador em memória (simples, mas inconsistente entre réplicas); rate limiting no ingress (centraliza, mas não conhece usuário/API key). O limitador na aplicação permite evoluir para escopos autenticados sem trocar o ingress.

### 10. OpenAPI 3.1 em `api/openapi.yaml`, versionado em `/v1`

O contrato público fica na raiz em `api/openapi.yaml`, com base path `/v1`. C1 expõe `GET /v1/status` para provar o caminho contrato → cliente → frontend. `/healthz`, `/readyz` e `/metrics` são operacionais e ficam fora do contrato público. O CI roda `redocly lint` e `oasdiff breaking` contra a `main`; o frontend consome um cliente TypeScript gerado em `frontend/packages/api-client`.

**Alternativas:** contrato gerado a partir do código (menos controle sobre breaking changes); contrato sem cliente gerado (mais divergência entre frontend e API).

### 11. Um frontend no C1; backoffice depois

C1 cria `frontend/apps/web` com Next.js App Router, TypeScript e `pnpm`. `seller` e `admin` compartilham um app `backoffice` em C6/C7; os hostnames ficam reservados no ingress e respondem 404 até o app existir. O web faz fetch server-side para a API, evitando CORS no caminho principal.

**Alternativas:** três apps agora (mais deployables, mais build, mais RAM, sem ganho no fluxo crítico); um único app com todas as rotas (mais simples, mas mistura públicos e superficies de backoffice).

### 12. CI em dois workflows, sem publicação no merge

`ci.yml` roda **somente** em `pull_request` e cobre os quatro itens do contrato do projeto (testes, lint, typecheck, análise estática), mais OpenAPI, build, SBOM e scan de container. `release.yml` roda **somente** em push para `main`, com `contents: write`, calcula o bump semver e cria a tag anotada; se o HEAD já estiver marcado, não faz nada. A primeira tag é `v1.0.0`, e o bump segue Conventional Commits (breaking → major, feature → minor, correção → patch).

Não há build, assinatura ou push de imagem no merge. O PRD prevê isso na seção 15.1, mas a regra de CI do projeto limita o merge à tag; publicação e assinatura ficam para a change de prontidão para produção.

**Alternativas:** `semantic-release` (faz tag e publicação, mas mistura responsabilidades); checks também no merge (redundante e proibido pela regra).

### 13. Kustomize + Helm + Tilt, sem Argo CD local

Manifestos do projeto usam Kustomize com `base/` e `overlays/local/`; componentes de terceiros usam Helm; o inner loop usa Tilt e `kind load docker-image`. Argo CD local fica de fora porque consome RAM e não é necessário para o dia a dia.

**Alternativas:** manifests crus (menos estrutura para overlays futuros); Argo CD local (mais fiel à produção, mas caro e lento no ciclo local).

### 14. Bootstrap do repositório e licença

C1 inicializa o Git, adiciona `.gitignore` (segredos, builds, `.env.local`), o `LICENSE` MIT exigido pelo projeto e ADRs em `docs/adr/`. O remote GitHub é pré-requisito para o CI; o workflow é criado apontando para o repositório que for definido.

## Risks / Trade-offs

- **RAM/disk insuficientes** → instalar stores por necessidade, quotas desde o início, retenção de 24 h, sampling de traces e limpeza periódica de imagens.
- **Operador do CloudNativePG consome mais do que o previsto** → medir após a instalação; fallback documentado para StatefulSet puro.
- **Cold start acima de 10 min por pull de imagens** → pré-carregar imagens com `kind load` e cache local; medir o primeiro `make dev-up`.
- **Conflito nas portas 8080/8443** → o script de subida falha cedo com mensagem explícita, em vez de criar um ingress inacessível.
- **Corrida na criação da tag** → `concurrency` no workflow de release e verificação idempotente do HEAD antes de taggear.
- **Divergência com o PRD sobre build/assinatura no merge** → regra de CI do projeto prevalece; publicação e assinatura são explicitamente adiadas para C9.
- **Rate limiting fail-open durante indisponibilidade do Redis** → métrica e log de degradação; endpoints de autenticação poderão usar fail-closed em C2.
- **Jaeger em memória perde traces ao reiniciar** → aceito no ambiente local; produção usa backend persistente.

## Migration Plan

Não há dados nem aplicação existentes; é um bootstrap greenfield. A ordem de implantação é:

1. Inicializar Git, `.gitignore`, `LICENSE` e estrutura de diretórios.
2. Criar o cluster `mercado-dev`, namespaces, quotas e ingress.
3. Instalar PostgreSQL, Redis e o stack de observabilidade.
4. Subir o esqueleto Go (API/worker) com health, config, middlewares e `/v1/status`.
5. Aplicar migrations e rodar o seed de referência.
6. Subir o frontend `web` consumindo o cliente gerado.
7. Ativar Tilt, Makefile e os workflows de CI/release.
8. Registrar ADRs e validar o smoke test de ponta a ponta.

O rollback de C1 é destrutivo apenas no ambiente local: `make dev-down` remove os recursos do projeto e o cluster `mercado-dev`. Não há impacto em produção, porque nenhum manifesto desta change é promovido.

## Open Questions

- Qual será o owner e o nome do repositório GitHub? O remote é necessário para habilitar o CI; os workflows podem ser criados antes, mas só rodam depois que o repositório existir.
- Qual titular de copyright deve constar no `LICENSE`? Até a definição, o arquivo usa "Mercado contributors".
- Quais versões exatas de Go e Next.js serão pinadas? O PRD exige Go 1.22+ e App Router; a versão estável será fixada no primeiro commit e registrada em ADR.
- O CloudNativePG cabe no orçamento local sem comprometer os stores seguintes? A medição acontece na implementação; o fallback já está desenhado.
