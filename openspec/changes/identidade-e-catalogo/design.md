# Design

## Context

Ver `proposal.md` para a motivação. O estado atual relevante:

- A API é um monólito Go com `chi`, `pgx/v5`, Redis e middleware de correlação, log, métricas, rate limit e recuperação em `backend/internal/httpapi`.
- Não há camada de domínio: apenas handlers de plataforma e o seed de dados de referência.
- As migrations usam goose embutido e estratégia expand/contract (`backend/migrations`).
- O cliente TypeScript é gerado de `api/openapi.yaml` por `openapi-typescript` e hoje só expõe `getStatus`, `getHealth` e `getReadiness`.
- O frontend é Next.js 16 com App Router, React Server Components e CSS global próprio; não há Tailwind nem biblioteca de componentes.
- O ambiente local tem PostgreSQL, Redis e observabilidade, mas não tem MinIO, sessão de usuário nem catálogo.

Restrições: cluster local com orçamento de RAM (~9,5 GB disponível), sem Meilisearch/Redpanda nesta fase, e a regra de CI do projeto (PR valida, merge apenas cria tag).

## Goals / Non-Goals

**Goals:**

- Entregar a primeira fatia vertical comprável: conta, anúncio publicado, busca e detalhe do produto.
- Manter o backend como monólito modular, com domínio isolado do transporte HTTP.
- Garantir autorização no backend, sessão segura e validação de upload.
- Deixar o catálogo pronto para receber carrinho e checkout sem refazer o modelo de dados.
- Manter o contrato OpenAPI como fonte do cliente TypeScript.

**Non-Goals:**

- OIDC, MFA, passkeys, recuperação de senha e confirmação de email.
- Separação dos apps `seller` e `admin`; a gestão de anúncios fica em `/vendedor` no app `web`.
- Meilisearch/OpenSearch, busca facetada com contadores e recomendação.
- Carrinho, checkout, pedido, pagamento, email e reputação.
- Tailwind/shadcn: a UI continua em CSS global neste change.
- Upload assinado, CDN e moderação automática.

## Decisions

### 1. Monólito modular com pacotes de domínio

Criar `internal/identity` e `internal/catalog`, cada um com tipos de domínio, regras e repositório em `pgx`, e expor handlers em `internal/httpapi` que apenas traduzem HTTP. O `httpapi.Server` recebe as dependências via `Deps`.

**Por quê:** mantém o padrão atual, evita um framework novo e permite extrair serviços depois sem reescrever regras. **Alternativas:** microserviços (complexidade prematura); colocar toda a lógica em handlers (dificulta teste e evolução).

### 2. Sessão opaca em PostgreSQL + Argon2id

Gerar token de sessão aleatório de 32 bytes, devolver em cookie e persistir apenas SHA-256 do token em `sessions`. Senhas usam Argon2id com parâmetros definidos em código e salt aleatório por usuário. Sessão com validade absoluta de 7 dias, renovação do identificador a cada login e logout que apaga o registro.

**Por quê:** permite revogação imediata, não exige JWT nem Dex e cabe no PostgreSQL já disponível. **Alternativas:** JWT (revogação difícil sem lista negra); Dex/Keycloak (consome ~0,5 GB e adiciona complexidade de setup).

### 3. Papéis em tabela `user_roles`

Usar `user_roles(user_id, role)` em vez de uma coluna única, porque um vendedor continua sendo comprador. `buyer` é atribuído no cadastro; `seller` é adicionado no onboarding; `admin` só é criado pelo seed/operador.

**Por quê:** evita migração destrutiva quando papéis adicionais surgirem (ex.: suporte, risco). **Alternativas:** coluna enum única (não modela papéis cumulativos).

### 4. IDs UUIDv7 gerados na aplicação

Usar `github.com/google/uuid` com `uuid.NewV7()` para usuários, sessões, categorias, anúncios, imagens e eventos. O PostgreSQL armazena `uuid`.

**Por quê:** segue o PRD, melhora localidade de índice e facilita sharding futuro por `cell_id` sem depender de sequência. **Alternativas:** `bigserial` (simples, mas sem portabilidade para cells); ULID (equivalente, exigiria outra dependência).

### 5. Máquina de estados do anúncio com auditoria separada

`listings.status` é `text` com `CHECK` nos valores `draft`, `pending_review`, `published`, `rejected`, `suspended`, `archived`. As transições são validadas no domínio. Cada decisão administrativa gera um registro imutável em `listing_moderation_events` com ator, ação, motivo e data; o anúncio guarda apenas o último estado e os campos `published_at` e `submitted_at`.

**Por quê:** separa estado atual de histórico, atende auditoria e permite reconstruir a linha do tempo. **Alternativas:** colunas soltas de moderação (perde histórico); tabela de eventos como fonte única (consulta pública mais cara).

### 6. Imagens em MinIO com chaves UUID e bucket público de leitura

Adicionar MinIO ao namespace `mercado-data` com PVC de 1 GiB. O bucket `mercado-listings` tem leitura pública local; cada objeto usa chave `listings/<listing_id>/<uuid>.<ext>`. A API valida o conteúdo pelos primeiros bytes (`http.DetectContentType`) e grava metadados em `listing_images`. A primeira imagem (menor `position`) é a capa.

**Por quê:** imagens de produto são conteúdo público, e chaves não adivinháveis evitam enumeração. **Alternativas:** proxy pela API (consome banda e conexões); signed URLs (melhor em produção, mas desnecessário localmente). Em produção, trocar por bucket privado + CDN assinada.

### 7. Estoque com linha bloqueada e log de ajustes

`inventory` tem uma linha por anúncio com `quantity` e `version`; ajustes usam transação com `SELECT ... FOR UPDATE`, atualizam a quantidade e inserem `inventory_adjustments` com delta, motivo e autor. A quantidade nunca fica negativa. Reservas entram no change de checkout como coluna/estrutura adicional.

**Por quê:** consistência forte exigida pelo PRD e base pronta para reserva. **Alternativas:** atualizar direto em `listings` sem log (perde rastreabilidade e concorrência).

### 8. Busca em PostgreSQL com `unaccent` + `pg_trgm`

Manter `listings.search_text` (título + descrição normalizados em minúsculas e sem acento) atualizado pela aplicação. Criar índice GIN trigram em `search_text` e B-tree em `published_at`, `price_cents` e `category_id`. Relevância usa `similarity(title, termo)` com desempate por `published_at DESC, id`. Categorias usam `parent_id` e a busca por categoria inclui descendentes via CTE recursiva.

**Por quê:** atende busca tolerante a acento/typo e filtros sem operar mais um componente. **Alternativas:** Meilisearch (melhor relevância, mas +RAM e novo data plane); `ILIKE` puro (sem tolerância e degrada com volume).

### 9. Paginação por offset

A API expõe `page` e `page_size` (padrão 20, máximo 50) e retorna `total` e `total_pages`. A ordenação sempre inclui `id` como desempate.

**Por quê:** simplicidade e alinhamento com a spec; o volume local é pequeno. **Alternativas:** cursor (melhor para escala, mas muda o contrato e a UI).

### 10. Cookie first-party via rewrite do Next

O frontend chama a API por `/api/:path*`, com rewrite do Next para `API_BASE_URL` (`http://127.0.0.1:8080` em dev e `http://api.mercado:8080` no cluster). O cookie de sessão fica no domínio do `web`, evitando CORS e cookie cross-subdomain. Leituras em Server Components continuam chamando a API diretamente no servidor; mutações e login usam o proxy same-origin.

**Por quê:** resolve `SameSite=Lax` e CORS sem abrir exceções. **Alternativas:** cookie em `.mercado.localhost` (funciona, mas exige CORS e configuração de domínio); BFF separado (mais uma camada agora).

### 11. CSRF e brute force

Além de `SameSite=Lax`, requisições mutantes com cookie MUST validar `Origin`/`Sec-Fetch-Site` contra a origem do `web`. Login tem limite adicional por conta+IP usando Redis. Mensagens de erro de login são genéricas e o hash é calculado mesmo para email inexistente.

**Por quê:** o cookie é a credencial e precisa de proteção explícita. **Alternativas:** double-submit token (mais peças na UI); confiar só em `SameSite` (insuficiente para navegadores antigos e para o proxy).

### 12. Contrato primeiro e cliente com mutações

Atualizar `api/openapi.yaml` antes da implementação, regenerar `frontend/packages/api-client/src/schema.ts` e estender `src/index.ts` com `post`, `patch`, `delete` e `credentials: "include"`, mantendo os tipos gerados como fonte única.

**Por quê:** preserva o fluxo já existente e evita divergência entre backend e frontend. **Alternativas:** cliente escrito à mão (duplicação e drift).

### 13. Seed determinístico com credenciais por ambiente

`cmd/seed` passa a criar categorias, um vendedor de demonstração e anúncios publicados. Credenciais de admin e vendedor vêm de `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`, `SEED_SELLER_EMAIL` e `SEED_SELLER_PASSWORD`; sem elas, o seed cria apenas categorias. Senhas nunca são logadas nem versionadas. Anúncios de seed não terão imagens; a UI usa placeholder.

**Por quê:** mantém o seed idempotente e reproduzível sem credenciais fixas no repositório. **Alternativas:** senha fixa (`demo123`) — simples, porém vira hábito inseguro e dispara scanners.

### 14. Observabilidade de negócio

Adicionar contadores de cadastro, login falho, anúncios por estado e buscas; traces já instrumentam `pgx`. Eventos de moderação e mudanças de estoque são auditáveis em banco, não apenas em logs.

**Por quê:** permite validar o funil e investigar abuso. **Alternativas:** apenas logs (difícil agregar).

## Risks / Trade-offs

- [Sessão em PostgreSQL adiciona carga de leitura por request] → índice por `token_hash`, expiração e limpeza periódica no worker; volume local é baixo.
- [Bucket público permite acesso a imagem de anúncio não publicado se a chave vazar] → chaves UUID não sequenciais e nenhuma listagem pública de objetos; em produção, bucket privado + CDN assinada.
- [Busca trigram degrada com catálogo grande] → índice GIN, limite de resultados e caminho claro para Meilisearch quando o catálogo passar de dezenas de milhares.
- [Offset pagination fica lento em páginas profundas] → limite de página e migração para cursor quando houver volume.
- [MinIO consome RAM no cluster local] → limites de 256 MiB e PVC de 1 GiB.
- [CSS global sem Tailwind aumenta custo de UI] → componentes pequenos e reutilizáveis em `packages/ui`; migração de UI fica para change própria.
- [Seed com credenciais por env pode ser esquecido] → documentar em `docs/local-data.md` e falhar de forma explícita no E2E quando as variáveis não existirem.
- [Moderação manual bloqueia publicação] → anúncios de seed já publicados e fila administrativa documentada; auto-publicação não entra agora.

## Migration Plan

1. Adicionar migration expand com novas tabelas, extensões `pg_trgm` e `unaccent`, e índices; não alterar tabelas existentes.
2. Adicionar MinIO e o secret correspondente; subir `make dev-up` e aplicar migrations.
3. Rodar o seed com as variáveis de ambiente locais.
4. Implantar a API e depois o web; o contrato novo é aditivo, então clientes antigos continuam funcionando.
5. Rollback: reverter imagens de API/web; as tabelas novas podem permanecer sem afetar a fundação. Para limpeza completa, rodar as migrations `down` na ordem inversa e remover o PVC do MinIO.

## Open Questions

- Em produção, as imagens devem ser servidas por CDN assinada ou por proxy autenticado da API? A decisão não muda o contrato nem as tasks deste change.
