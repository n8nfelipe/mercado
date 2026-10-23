# Proposal

## Why

O projeto entregou apenas a fundação de plataforma (health checks, dados de referência e observabilidade). Nenhum requisito funcional do PRD está implementado: não existe conta de usuário, catálogo, busca nem página de produto. Sem identidade e catálogo não é possível iniciar o funil de compra nem as fatias seguintes (carrinho, checkout, pedidos).

Esta change entrega a primeira fatia vertical do marketplace: um vendedor consegue se cadastrar, publicar um anúncio com imagem e estoque, e um comprador consegue se cadastrar, buscar, filtrar e abrir a página do produto.

## What Changes

- Criar contas de usuário com email/senha, sessão em cookie HttpOnly e papéis `buyer`, `seller` e `admin`.
- Criar onboarding de vendedor com perfil obrigatório antes de publicar anúncios.
- Criar categorias administradas por admin e seed inicial de categorias.
- Criar CRUD de anúncios do vendedor com título, descrição, categoria, preço, condição, estoque e imagens.
- Adicionar upload de imagens no MinIO local, com validação de tipo e tamanho.
- Criar moderação de anúncios com os estados `draft`, `pending_review`, `published`, `rejected` e `suspended`.
- Criar busca pública com paginação, filtros (categoria, preço, condição, frete grátis) e ordenação (relevância, menor preço, mais recentes), refletidos na URL.
- Criar as páginas do comprador: home com vitrine, busca, detalhe do produto, cadastro e login.
- Adicionar uma área de vendedor dentro do app `web` para gestão de anúncios e estoque; a separação no app `seller` e a UI de administração ficam para changes posteriores.
- Adicionar MinIO ao stack local e ao Kustomize, além das variáveis de configuração correspondentes.
- Estender o contrato OpenAPI e o cliente TypeScript gerado para as novas rotas.
- Adicionar testes de integração dos fluxos de identidade, publicação e busca, e testes E2E do caminho comprador.

Fora do escopo desta change: carrinho, checkout, pagamento, pedidos, notificações por email, avaliações, reputação, app separado de vendedor/admin, Meilisearch e busca por relevância avançada. A busca usa PostgreSQL com `pg_trgm` até haver volume que justifique um índice dedicado.

## Capabilities

### New Capabilities

- `identity/accounts`: cadastro, login, logout, sessão, papéis, autorização por recurso e onboarding de vendedor.
- `catalog/listing-management`: categorias, anúncios, imagens, estoque, moderação e publicação pelo vendedor e pelo admin.
- `catalog/discovery`: vitrine pública, busca, filtros, ordenação, paginação e detalhe do produto.

### Modified Capabilities

Nenhuma. O projeto ainda não possui specs consolidadas em `openspec/specs/`.

## Impact

- **Backend:** novos pacotes de domínio e persistência para identidade e catálogo; novos handlers e rotas sob `/v1`; configuração de MinIO, sessão e senha.
- **Banco:** novas tabelas `users`, `sessions`, `seller_profiles`, `categories`, `listings`, `listing_images` e `inventory`; extensão `pg_trgm`; migrations expand/contract.
- **Infra local:** MinIO no namespace de dados, bucket de imagens e secrets; novos valores em `.env.example`.
- **Frontend:** páginas públicas de vitrine, busca, produto, login e cadastro, além da área `/vendedor`; novos componentes em `packages/ui`.
- **Contratos:** `api/openapi.yaml` e `frontend/packages/api-client` passam a cobrir identidade, catálogo e busca.
- **Seed:** categorias iniciais, vendedor de demonstração e anúncios sintéticos determinísticos para o ambiente local.
- **Testes:** integração com PostgreSQL real para identidade/catálogo e Playwright para o funil de descoberta.
- **Dependências:** MinIO no ambiente local; `golang.org/x/crypto` para Argon2id e `github.com/google/uuid` para identificadores.
