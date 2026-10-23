# Tasks

## 1. Contrato, cliente e schema

- [x] 1.1 Estender `api/openapi.yaml` com os schemas e rotas de identidade, categorias, anúncios, imagens, estoque, busca e detalhe público, incluindo códigos de erro; verificar com `pnpm dlx @redocly/cli@1.34.2 lint api/openapi.yaml`.
- [x] 1.2 Regenerar `frontend/packages/api-client/src/schema.ts`, adicionar métodos `post`, `patch` e `delete` com `credentials: "include"` e tipos exportados; verificar com `pnpm --filter @mercado/api-client build` e `typecheck`.
- [x] 1.3 Criar a migration expand de identidade (`users`, `user_roles`, `sessions`, `seller_profiles`) com constraints, índices por email normalizado e hash de sessão; verificar `up` e `down` em banco efêmero e teste de unicidade de email.
- [x] 1.4 Criar a migration expand de catálogo (`categories`, `listings`, `listing_images`, `inventory`, `inventory_adjustments`, `listing_moderation_events`), extensões `pg_trgm`/`unaccent` e índices de busca; verificar `up` e `down` e presença dos índices com `pg_indexes`.
- [x] 1.5 Documentar as novas rotas e o fluxo de regeneração em `docs/api-contract.md`; verificar que os comandos documentados executam como escritos.

## 2. Identidade e sessão

- [x] 2.1 Implementar hash e verificação de senha com Argon2id e parâmetros explícitos; verificar com testes unitários de hash, verificação e rejeição de senha incorreta.
- [x] 2.2 Implementar repositórios de usuários, papéis e sessões com `pgx` e IDs UUIDv7; verificar com testes de integração em PostgreSQL efêmero, incluindo expiração e revogação.
- [x] 2.3 Implementar cadastro, login, logout e `GET /v1/auth/me` com cookie `HttpOnly`/`SameSite=Lax` e mensagens genéricas; verificar com testes de integração de sucesso, email duplicado, senha fraca e credenciais inválidas.
- [x] 2.4 Implementar middleware de autenticação, contexto de usuário e `requireRole`; verificar com testes que rotas protegidas retornam `401` sem sessão e `403` sem papel.
- [x] 2.5 Implementar validação de origem em requisições mutantes e limite de tentativas de login por conta+IP no Redis; verificar com testes de origem inválida, `429` com `Retry-After` e ausência de enumeração de email.
- [x] 2.6 Implementar onboarding de vendedor, perfil público e regra de documento único; verificar com testes de sucesso, documento duplicado, campos ausentes e acesso ao perfil de terceiros.
- [x] 2.7 Documentar variáveis de sessão, cookies, CSRF e ciclo de vida da sessão em `docs/runtime.md`; verificar que os comandos de exemplo funcionam.

## 3. Categorias e anúncios

- [x] 3.1 Implementar repositório e handlers administrativos de categorias com slug único, categoria pai, ativação e bloqueio de remoção em uso; verificar com testes de CRUD, `409` e listagem pública de categorias ativas.
- [x] 3.2 Implementar criação e edição de anúncio com validação de título, descrição, preço, moeda, condição, frete e estoque inicial; verificar com testes de cada campo inválido e de criação `201` em `draft`.
- [x] 3.3 Implementar a máquina de estados do anúncio e as regras de edição por estado e por proprietário; verificar com testes de transições válidas, transições inválidas e edição por terceiro.
- [x] 3.4 Implementar submissão, rejeição, publicação, suspensão e arquivamento, registrando `listing_moderation_events` com ator e motivo; verificar com testes de cada transição e de auditoria.
- [x] 3.5 Implementar as rotas do vendedor para listar e detalhar seus anúncios e a fila de moderação administrativa; verificar com testes de isolamento entre vendedores e ordenação da fila.
- [x] 3.6 Adicionar métricas de anúncios por estado e de decisões de moderação; verificar que as séries aparecem no `/metrics` após exercitar as rotas.

## 4. Estoque e imagens

- [x] 4.1 Implementar estoque com transação e `SELECT ... FOR UPDATE`, atualização de quantidade e registro de ajustes; verificar com teste de concorrência e de tentativa de estoque negativo.
- [x] 4.2 Adicionar MinIO ao Kustomize, secrets locais, variáveis de configuração e cliente de storage; verificar que o pod fica `Ready` e que o bucket `mercado-listings` existe após `make dev-up`.
- [x] 4.3 Implementar upload de imagem com validação por magic bytes, limite de 5 MiB, máximo de 8 imagens e gravação de metadados; verificar com testes de sucesso, `413`, `415` e `409`.
- [x] 4.4 Implementar remoção, ordenação e definição da imagem de capa, garantindo que só o proprietário altere; verificar com testes de autorização e de ordenação pública.
- [x] 4.5 Documentar MinIO, bucket, reset e variáveis em `docs/local-data.md`; verificar que o procedimento documentado funciona do zero.

## 5. Busca e descoberta

- [x] 5.1 Manter `search_text` normalizado na escrita e indexado com `pg_trgm`; verificar com teste que busca sem acento encontra título acentuado.
- [x] 5.2 Implementar filtros combináveis, ordenação determinística e paginação no repositório; verificar com testes de cada filtro, ordenação, tamanho máximo e página fora do intervalo.
- [x] 5.3 Implementar `GET /v1/catalog/listings` e `GET /v1/listings/{id}` expondo apenas anúncios publicados de categorias ativas; verificar com testes de visibilidade, `404` para não público e omissão de dados privados do vendedor.
- [x] 5.4 Adicionar métricas de busca, filtros e latência por rota; verificar que buscas sem resultado e detalhes `404` são observáveis no Prometheus e nos traces.

## 6. Frontend do comprador

- [x] 6.1 Configurar rewrite `/api/:path*` para a API, ajustar `API_BASE_URL` e garantir encaminhamento de cookie; verificar com `curl` através do web que login define cookie e `me` retorna o usuário.
- [x] 6.2 Criar componentes de layout, cabeçalho, card de anúncio, filtros e paginação em `packages/ui`; verificar com build e typecheck do pacote e uso na página inicial.
- [x] 6.3 Implementar home com vitrine, busca textual, filtros, ordenação e paginação refletidos na URL; verificar com Playwright que aplicar filtro altera a URL e recarregar restaura o resultado.
- [x] 6.4 Implementar a página de detalhe do produto com galeria, preço, condição, estoque e vendedor; verificar com Playwright que anúncio publicado abre e anúncio suspenso retorna não encontrado.
- [x] 6.5 Implementar páginas de cadastro, login e logout com validação de formulário e mensagens de erro; verificar com Playwright o fluxo completo incluindo sessão persistente.
- [x] 6.6 Documentar rotas, rewrite e fluxo de sessão no frontend em `docs/frontend.md`; verificar que os comandos documentados executam.

## 7. Área do vendedor

- [x] 7.1 Criar a área `/vendedor` protegida por papel, com lista de anúncios e estados; verificar com Playwright que comprador é redirecionado e vendedor vê apenas seus anúncios.
- [x] 7.2 Implementar formulário de criação e edição de anúncio com validação e submissão para revisão; verificar com Playwright que o anúncio aparece como `pending_review`.
- [x] 7.3 Implementar upload, remoção e ordenação de imagens no formulário; verificar com Playwright que a capa escolhida aparece na página pública após publicação.
- [x] 7.4 Implementar ajuste de estoque com motivo e exibição do histórico; verificar com Playwright que estoque zero mostra indisponibilidade no detalhe público.
- [x] 7.5 Exibir motivo de rejeição e permitir reenviar o anúncio para revisão; verificar com Playwright o ciclo rejeitado → reenviado.

## 8. Seed e dados locais

- [x] 8.1 Estender `cmd/seed` com categorias, usuários e anúncios determinísticos a partir de variáveis de ambiente, sem logar senhas; verificar que duas execuções não duplicam registros e que sem as variáveis apenas categorias são criadas.
- [x] 8.2 Atualizar `scripts/seed.sh`, `.env.example` e a documentação de seed em `docs/migrations.md`; verificar `make seed` em banco migrado e a criação de anúncios publicados.
- [x] 8.3 Atualizar `scripts/dev-secrets.sh` com as credenciais do MinIO e documentar a rotação; verificar que nenhum valor aparece no Git nem nos logs.

## 9. Verificação integrada

- [ ] 9.1 Executar `make test` e `make lint-api`; verificar que testes Go, typecheck, testes de frontend e validação OpenAPI passam.
- [ ] 9.2 Recriar o ambiente com `make dev-up`, rodar migrations, seed e `make smoke`; verificar que identidade, catálogo e busca respondem no cluster.
- [ ] 9.3 Rodar o E2E Playwright do funil comprador (cadastro → busca → produto) e do vendedor (publicar → aprovar → buscar); verificar que os dois passam contra o ambiente local.
- [ ] 9.4 Ajustar `scripts/load.js` com cenário de busca e detalhe e executar `make load`; verificar p95 de busca < 300 ms e erro < 1% na carga local.
- [ ] 9.5 Verificar no Grafana/Jaeger as métricas e traces de login, busca e moderação, e confirmar que senha, cookie e documento do vendedor não aparecem em logs; registrar o resultado em `docs/verification.md`.
- [ ] 9.6 Rodar `gitleaks detect` e `govulncheck ./...`; verificar que não há segredo versionado nem vulnerabilidade crítica conhecida nas dependências novas.

## Workflow follow-up

- Arquivar a change depois que os requisitos de review do projeto forem satisfeitos.
- Verificar o resultado arquivado e confirmar que as capabilities foram consolidadas em `openspec/specs/`.
