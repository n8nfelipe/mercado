# Tasks

## 1. Bootstrap do repositório

Specs: `platform/local-environment`, `platform/delivery`, `platform/api-contract`.
Design: decisões 12, 13 e 14.

- [x] 1.1 Inicializar o repositório Git e criar `.gitignore` cobrindo `.env.local`, segredos, binários, `node_modules`, artefatos de build e caches; verificar com `git status --ignored` que esses caminhos são ignorados.
- [x] 1.2 Criar a estrutura `backend/cmd/{api,worker,seed}`, `backend/internal/platform`, `backend/migrations`, `frontend/apps/web`, `frontend/packages/{ui,api-client}`, `deploy/kind`, `deploy/k8s/{base,overlays/local}`, `scripts` e `docs/adr`; verificar com `find` que todos os diretórios existem.
- [x] 1.3 Adicionar `LICENSE` MIT com o titular "Mercado contributors"; verificar que o arquivo existe e que `git diff --check` não acusa problemas.
- [x] 1.4 Registrar ADR-0001 (cluster e portas), ADR-0002 (data plane local) e ADR-0003 (regra de CI prevalecendo sobre o PRD); verificar que os três arquivos existem em `docs/adr/` e citam as decisões do `design.md`.
- [x] 1.5 Configurar o remote GitHub quando owner e nome forem definidos; verificar com `git remote -v`. Se ainda não estiver definido, registrar a pendência em `docs/ci.md` sem inventar URL.

## 2. Cluster, ingress e governança de recursos

Specs: `platform/local-environment`.
Design: decisões 1 e 2.

- [x] 2.1 Escrever `deploy/kind/cluster.yaml` com um control-plane, mapeamento das portas 8080/8443 e label `ingress-ready=true`; verificar criando o cluster e rodando `kubectl get nodes`.
- [x] 2.2 Instalar ingress-nginx com o provider `kind`; verificar que uma requisição a `api.mercado.localhost:8080` chega ao backend padrão e que um host não declarado não é roteado para serviço do projeto.
- [x] 2.3 Criar os namespaces `mercado`, `mercado-data` e `mercado-observability`, com `ResourceQuota` e `LimitRange` com defaults explícitos; verificar com `kubectl describe quota,limitrange -n mercado` que um pod sem requests/limits recebe os defaults e que um pod acima do máximo é rejeitado.
- [x] 2.4 Implementar `scripts/dev-up.sh`, `scripts/dev-down.sh` e os alvos `make dev-up`/`make dev-down`; verificar que duas execuções consecutivas de `make dev-up` terminam com sucesso sem duplicar recursos.
- [x] 2.5 Adicionar preflight de portas 8080/8443 no script de subida; verificar que, com a porta ocupada, o script falha com mensagem explícita antes de criar recursos.
- [x] 2.6 Documentar topologia, hostnames e limitações locais em `docs/local-environment.md`; verificar que os comandos documentados executam como escritos.

## 3. Dados locais

Specs: `platform/local-environment`, `platform/data`.
Design: decisões 3 e 8.

- [x] 3.1 Instalar o operador CloudNativePG via Helm; verificar que os CRDs existem e que o pod do operador fica `Ready`.
- [x] 3.2 Criar o recurso `Cluster` do PostgreSQL com uma instância, StorageClass `local-path` e requests/limits; verificar com `kubectl get cluster` que o status fica saudável e conectar com `psql`.
- [x] 3.3 Criar Redis como StatefulSet de um pod com Service e PVC; verificar com `redis-cli ping` a partir de um pod do namespace `mercado`.
- [x] 3.4 Implementar `scripts/dev-secrets.sh` para gerar credenciais locais e aplicar Kubernetes Secrets; verificar que os pods consomem os Secrets e que `git grep` não encontra os valores no repositório.
- [x] 3.5 Testar persistência gravando um registro no PostgreSQL, removendo o pod e confirmando o registro após o pod voltar; verificar também que o PVC do Redis permanece após reinício.
- [x] 3.6 Documentar o data plane e o fallback para PostgreSQL em StatefulSet em `docs/local-data.md`; verificar que o procedimento de conexão e reset documentado funciona.

## 4. Observabilidade

Specs: `platform/observability`.
Design: decisão 4.

- [x] 4.1 Instalar Prometheus, Grafana, Jaeger all-in-one, kube-state-metrics e node-exporter com requests/limits e retenção de 24 h; verificar que todos os pods ficam `Ready`.
- [x] 4.2 Provisionar fontes de dados e dashboards do Grafana como ConfigMaps; verificar que os dashboards aparecem após recriar o pod do Grafana, sem configuração manual.
- [x] 4.3 Configurar a exportação OTLP para o Jaeger e validar que a UI/API do Jaeger responde; verificar a chegada de um span sintético de teste.
- [x] 4.4 Documentar URLs locais, retenção e limitações do stack em `docs/observability.md`; verificar que os comandos de acesso documentados funcionam.

## 5. Backend, runtime e contrato de status

Specs: `platform/runtime`, `platform/observability`, `platform/api-contract`.
Design: decisões 5, 8, 9 e 10.

- [x] 5.1 Criar o módulo Go, o router chi e o pacote de configuração com validação de variáveis obrigatórias; adicionar `.env.example` sem segredos e verificar com `go build ./...` e testes de configuração inválida.
- [x] 5.2 Implementar `/healthz` (liveness) e `/readyz` (PostgreSQL e Redis) distintos; verificar com testes que a queda do PostgreSQL deixa `/healthz` 200 e `/readyz` indisponível.
- [x] 5.3 Implementar encerramento gracioso em `SIGTERM` com timeout configurável; verificar com teste que uma requisição em andamento termina e que novas conexões são recusadas.
- [x] 5.4 Implementar correlation ID, access log em JSON e redação de campos sensíveis; verificar com testes que `Authorization`, cookies, senha e token não aparecem nos logs.
- [x] 5.5 Expor métricas RED em `/metrics` por rota e status; verificar que uma rota exercitada aparece nas séries e que uma rota sem tráfego não gera série falsa.
- [x] 5.6 Instrumentar traces OpenTelemetry para HTTP, PostgreSQL e Redis; verificar no Jaeger que o trace contém os spans e que o log correspondente contém o trace ID.
- [x] 5.7 Adicionar middleware de recuperação de panic; verificar com teste que o cliente recebe 500 e que o processo continua atendendo a requisição seguinte.
- [x] 5.8 Implementar rate limiting em Redis via Lua para IP e API key, com bypass de `/healthz`, `/readyz` e `/metrics` e métrica de fail-open; verificar 429 com `Retry-After`, isolamento entre chaves e comportamento com Redis indisponível.
- [x] 5.9 Expor `GET /v1/status` com serviço, versão, commit e status; verificar o JSON e a aderência ao contrato OpenAPI.
- [x] 5.10 Criar Dockerfile multi-stage, imagem não-root e healthcheck; verificar que a imagem constrói e responde a `/healthz` fora do cluster.
- [x] 5.11 Documentar runtime, variáveis e comportamento de degradação em `docs/runtime.md`; verificar que os comandos e exemplos documentados executam.

## 6. Migrations e seed

Specs: `platform/data`.
Design: decisões 6 e 7.

- [x] 6.1 Configurar goose com migrations embutidas e criar a migration inicial de países, moedas e locales; verificar `up` e `down` em banco vazio.
- [x] 6.2 Implementar `backend/cmd/seed` determinístico e idempotente para os dados de referência; verificar que duas execuções com a mesma semente não duplicam registros e que sementes diferentes geram conjuntos diferentes.
- [x] 6.3 Adicionar teste de integração com Testcontainers que aplica migrations e executa o seed; verificar que `go test` do pacote passa em banco efêmero.
- [x] 6.4 Adicionar o alvo `make seed` e documentar reset/rollback em `docs/migrations.md`; verificar que o procedimento documentado funciona do zero.
- [x] 6.5 Escrever um teste de rollback da última migration; verificar que o schema retorna à versão anterior sem perda incompatível.

## 7. Contrato OpenAPI e cliente gerado

Specs: `platform/api-contract`, `platform/delivery`.
Design: decisão 10.

- [x] 7.1 Criar `api/openapi.yaml` em OpenAPI 3.1 com `/v1/status`, envelope de erro e servidores locais; verificar com `redocly lint`.
- [x] 7.2 Configurar `oasdiff breaking` contra a `main`, ignorando a checagem quando a branch base ainda não existir; verificar que uma remoção de campo em branch sintética falha e que uma adição opcional passa.
- [x] 7.3 Gerar o cliente TypeScript em `frontend/packages/api-client` a partir do contrato; verificar que `pnpm build` do pacote consome os tipos gerados.
- [x] 7.4 Documentar o fluxo de alteração de contrato em `docs/api-contract.md`; verificar que lint, breaking check e geração descritos executam como escritos.

## 8. Frontend do comprador

Specs: `platform/local-environment`, `platform/api-contract`.
Design: decisões 10 e 11.

- [x] 8.1 Criar o app Next.js com App Router, TypeScript e workspace `pnpm`; verificar `pnpm install` e `pnpm build`.
- [x] 8.2 Criar `frontend/packages/ui` e a página inicial que consulta `GET /v1/status` server-side; verificar que a página exibe o status com a API no ar e uma mensagem de indisponibilidade com a API fora.
- [x] 8.3 Criar Dockerfile multi-stage não-root para o web; verificar que a imagem constrói e serve a página.
- [x] 8.4 Adicionar smoke test Playwright para a página inicial e o status; verificar que o teste passa contra o ambiente local.
- [x] 8.5 Documentar o app, o workspace e o fetch server-side em `docs/frontend.md`; verificar que os comandos documentados executam.

## 9. Deploy local com Kustomize, Tilt e Makefile

Specs: `platform/local-environment`, `platform/runtime`.
Design: decisão 13.

- [x] 9.1 Criar Kustomize `base/` e `overlays/local/` para API, web, worker, Services e Ingress; verificar com `kubectl kustomize` e `kubectl apply --dry-run=server`.
- [x] 9.2 Criar `Tiltfile` com `kind load docker-image` e live update; verificar que `tilt up` sobe os serviços e que uma alteração de código reinicia o pod correto.
- [x] 9.3 Adicionar os alvos `make test`, `make smoke` e `make load` (k6 com 1 VU); verificar que cada alvo executa e retorna o código esperado.
- [x] 9.4 Documentar o fluxo de deploy local, `make dev-up` e recuperação de falhas em `docs/local-deploy.md`; verificar que o runbook permite recriar o ambiente do zero.

## 10. CI e release

Specs: `platform/delivery`, `platform/api-contract`.
Design: decisão 12.

- [x] 10.1 Criar `.github/workflows/ci.yml` com **somente** `on: pull_request` e `permissions: contents: read`, cobrindo testes, lint, typecheck, análise estática, OpenAPI, build, SBOM e scan; verificar com `actionlint` e com a ausência de `push` para `main`.
- [x] 10.2 Criar `.github/workflows/release.yml` com **somente** `on: push: branches: [main]` e `permissions: contents: write`, calculando bump semver, começando em `v1.0.0`, pulando HEAD já marcado e usando `concurrency`; verificar com `actionlint` e testar a lógica de bump em repositório sintético em `/tmp`.
- [x] 10.3 Confirmar que `release.yml` não contém testes, lint, build ou publicação de imagem; verificar por inspeção do YAML e por `actionlint`.
- [x] 10.4 Documentar a regra de CI, a divergência deliberada com a seção 15.1 do PRD e o pré-requisito de remote em `docs/ci.md`; verificar que o documento cita os dois workflows e os quatro itens de verificação.

## 11. Verificação integrada

Specs: todas as capabilities da change.
Design: critérios de saída e riscos.

- [x] 11.1 A partir de um host limpo, executar `make dev-up` e registrar o tempo total; verificar que o ambiente fica pronto em menos de 10 minutos.
- [x] 11.2 Executar `make test` e `make smoke`; verificar que testes unitários, integração e smoke passam.
- [x] 11.3 Verificar manualmente `/healthz`, `/readyz`, `/metrics`, `/v1/status`, a página do comprador, um trace no Jaeger, um dashboard no Grafana e a presença do correlation ID na resposta e no log.
- [x] 11.4 Verificar quotas, encerramento gracioso, 429 do rate limit e readiness com Redis indisponível; registrar os resultados em `docs/verification.md`.
- [x] 11.5 Executar `make dev-down` e um novo `make dev-up`; verificar que o ambiente é reproduzível e que a política de persistência se comporta como documentada.
- [x] 11.6 Rodar `openspec validate fundacao-plataforma-local --strict` e `openspec status --change fundacao-plataforma-local --json`; verificar que os artefatos estão válidos e completos.
- [x] 11.7 Rodar uma varredura de segredos no repositório (por exemplo, `gitleaks detect`); verificar que nenhum segredo ou `.env.local` foi versionado.

## Workflow follow-up

- Arquivar a change depois que os requisitos de review do projeto forem satisfeitos.
- Verificar o resultado arquivado e confirmar que as capabilities foram consolidadas em `openspec/specs/`.
