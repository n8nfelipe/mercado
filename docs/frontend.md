# Frontend do comprador

## Estrutura

- `frontend/apps/web`: Next.js 16 com App Router, TypeScript e React 19.
- `frontend/packages/ui`: componentes compartilhados (`SiteHeader`, `ListingCard`, `CatalogFilters`, `Pagination`, `EmptyState`, `StatusCard`, `UnavailablePanel`).
- `frontend/packages/api-client`: cliente tipado gerado a partir de `api/openapi.yaml`.
- `pnpm-workspace.yaml`: workspace com `frontend/apps/*` e `frontend/packages/*`.

O browser fala apenas com a origem do web: `frontend/apps/web/middleware.ts` reescreve `/api/:path*` para `API_BASE_URL`, encaminhando cookies de sessão e evitando CORS. Server Components consultam `API_BASE_URL` diretamente.

## Desenvolvimento

```bash
pnpm install
pnpm --filter @mercado/web dev
```

O dev server escuta em `127.0.0.1:3000`. O backend deve estar em `http://127.0.0.1:8080` ou `API_BASE_URL` deve apontar para outro endereço.

## Build e execução local

```bash
pnpm -r build
pnpm --filter @mercado/web start
```

`start` prepara a saída `standalone` e executa `node .next/standalone/frontend/apps/web/server.js`. O Next 16 não recomenda `next start` com `output: "standalone"`.

Variáveis:

| Variável | Padrão | Uso |
|---|---|---|
| `API_BASE_URL` | `http://127.0.0.1:8080` | base do backend consultada no servidor e destino do proxy `/api` |
| `HOSTNAME` | `0.0.0.0` no container | endereço de escuta do standalone |
| `PORT` | `3000` | porta do standalone |

## Rotas

| Rota | Componente | Descrição |
|---|---|---|
| `/` | `app/page.tsx` | vitrine com busca, filtros, ordenação e paginação refletidos na URL |
| `/anuncio/{id}` | `app/anuncio/[listingId]/page.tsx` | detalhe público do anúncio, com galeria, preço, condição, estoque e vendedor |
| `/entrar` | `app/entrar/page.tsx` | formulário de login |
| `/cadastro` | `app/cadastro/page.tsx` | formulário de cadastro que autentica em seguida |
| `/vendedor` | `app/vendedor/page.tsx` | dashboard do vendedor com seus anúncios e status |
| `/vendedor/ativar` | `app/vendedor/ativar/page.tsx` | onboarding que concede o papel `seller` |
| `/vendedor/novo` | `app/vendedor/novo/page.tsx` | formulário de criação de anúncio |
| `/vendedor/{id}` | `app/vendedor/[listingId]/page.tsx` | edição, imagens, estoque, submissão e reenvio |

`requireSeller()` em `lib/session.ts` protege as rotas `/vendedor`: sem sessão redireciona para `/entrar?next=/vendedor`; sessão sem papel `seller` redireciona para `/vendedor/ativar`. O dashboard usa `listSellerListings()` no servidor com o cookie da sessão, então cada vendedor vê apenas seus anúncios. O editor usa `browserApi()` para editar, enviar para revisão, reordenar/remover imagens e ajustar estoque, respeitando as regras de estado do backend (por exemplo, só preço e estoque mudam após publicação).

A vitrine lê `searchParams` (`q`, `category_id`, `condition`, `free_shipping`, `min_price`, `max_price`, `sort`, `page`), converte preços em reais para centavos e chama `GET /v1/catalog/listings`. O `<form method="get">` dos filtros grava o estado na URL, então recarregar restaura o resultado. `anuncio` chama `GET /v1/listings/{id}` e usa `notFound()` quando a API responde `404`.

## Proxy same-origin

`middleware.ts` intercepta `/api/:path*` e reescreve para `API_BASE_URL` em runtime, removendo o prefixo `/api`. O cliente no browser usa `baseUrl: "/api"`, então `POST /api/v1/auth/login` grava o cookie `mercado_session` na origem do web e `GET /api/v1/auth/me` reenvia o cookie para a API.

```bash
# com o web em 127.0.0.1:3000 e a API acessível em API_BASE_URL
curl -i -c /tmp/mercado.jar -H 'content-type: application/json' \
  -d '{"email":"comprador@example.com","password":"senha-super-segura"}' \
  http://127.0.0.1:3000/api/v1/auth/login

curl -i -b /tmp/mercado.jar http://127.0.0.1:3000/api/v1/auth/me
```

O rewrite é resolvido em runtime, então trocar `API_BASE_URL` e reiniciar o web muda o destino sem rebuild. Requisições mutantes enviadas pelo browser carregam `Origin: http://<host-do-web>`, que precisa constar em `ALLOWED_ORIGINS` da API ou corresponder ao host da API.

## Fetch server-side

`app/page.tsx` é um Server Component dinâmico. A consulta usa `cache: "no-store"` e trata `ApiError` para renderizar uma mensagem de indisponibilidade em vez de falhar a página.

- API no ar: mostra serviço, versão e commit.
- API fora ou respondendo erro: mostra “API indisponível” e o status retornado.
- Nenhum token ou segredo é enviado ao browser.

## UI

`@mercado/ui` é compilado antes do app e expõe componentes puros: `SiteHeader`, `ListingCard`, `CatalogFilters`, `Pagination`, `EmptyState`, além de `StatusCard` e `UnavailablePanel`. Só os formulários de sessão (`SessionMenu`, `LoginForm`, `RegisterForm`) usam estado de cliente e `browserApi()` com `baseUrl: "/api"`. O CSS global fica em `frontend/apps/web/app/globals.css`.

## Docker

O build usa o contexto da raiz do monorepo:

```bash
docker build -f frontend/apps/web/Dockerfile -t mercado-web:local .
docker run --rm -p 18083:3000 \
  -e API_BASE_URL=http://host.docker.internal:8080 \
  mercado-web:local
```

A imagem é multi-stage, usa o usuário não-root `nextjs`, expõe `/healthz` e tem healthcheck próprio. Em Linux, para alcançar a API no host, use o IP da bridge ou uma rede Docker adequada em vez de `host.docker.internal`.

## Playwright

```bash
pnpm --filter @mercado/web test:e2e
```

Os testes usam uma API mock determinística em `127.0.0.1:18082` e cobrem:

1. vitrine com anúncios publicados e estado “API indisponível”;
2. filtro de categoria e busca textual refletidos na URL, incluindo recarregar e estado vazio;
3. detalhe publicado e anúncio suspenso retornando não encontrado;
4. cadastro com sessão persistente, logout e erro de credenciais inválidas;
5. `POST /api/v1/auth/login` e `GET /api/v1/auth/me` via proxy, com cookie de sessão;
6. fluxo do vendedor: comprador redirecionado, ativação da conta, criação, upload de capa, submissão, rejeição com motivo e reenvio, publicação, estoque zero indisponível.

Na primeira execução:

```bash
pnpm --filter @mercado/web exec playwright install chromium
```

## Build do workspace

```bash
pnpm -r build
pnpm --filter @mercado/api-client typecheck
pnpm --filter @mercado/ui typecheck
pnpm --filter @mercado/web typecheck
```
