# Contrato OpenAPI

## Fonte de verdade

`api/openapi.yaml` é o contrato público da API, em OpenAPI 3.1. Toda rota exposta pelo backend deve estar descrita nele, incluindo probes operacionais.

- Endpoints públicos usam o prefixo `/v1`.
- `GET /v1/status` retorna `service`, `version`, `commit` e `status`.
- Identidade: `POST /v1/auth/register`, `POST /v1/auth/login`, `POST /v1/auth/logout`, `GET /v1/auth/me` e `POST /v1/seller/onboarding`.
- Catálogo público: `GET /v1/categories`, `GET /v1/catalog/listings`, `GET /v1/listings/{listingId}` e `GET /v1/sellers/{sellerId}`.
- Vendedor: CRUD de `/v1/seller/listings`, submissão/arquivamento, imagens, ordenação e estoque.
- Administração: CRUD de categorias e moderação em `/v1/admin/listings`.
- Sessão usa o cookie `mercado_session` (`HttpOnly`, `SameSite=Lax`); operações mutantes exigem `Origin` válida.
- Erros usam `ErrorResponse`: `error.code`, `error.message`, `error.details` opcional e `correlation_id`.
- `429` inclui `Retry-After` e `X-Correlation-ID`.
- `X-API-Key` é opcional e serve apenas ao rate limit; não é autenticação nesta change.
- Os servidores locais são o ingress `http://api.mercado.localhost:8080` e a API direta `http://127.0.0.1:8080`.

## Validação

```bash
pnpm install
pnpm lint:api
```

O script `scripts/api-contract-check.sh`:

1. roda Redocly (`@redocly/cli@1.34.2`) com as regras de `.redocly.yaml`;
2. procura `main` ou `origin/main`;
3. se a referência existir, extrai `api/openapi.yaml` dela e roda `oasdiff breaking` com `--fail-on ERR`;
4. se a referência não existir, registra o skip e termina com sucesso.

Em um repositório sem `main`, a checagem de breaking é ignorada de propósito. Ela passa a valer assim que a branch base existir.

Variáveis úteis:

```bash
API_BASE_REF=origin/main scripts/api-contract-check.sh
API_BASE_FILE=/tmp/openapi-base.yaml scripts/api-contract-check.sh
API_REVISION_FILE=/tmp/openapi-revision.yaml scripts/api-contract-check.sh
```

`oasdiff` é executado via `go run github.com/tufin/oasdiff@v1.10.0`, sem instalação global.

## Mudanças compatíveis e incompatíveis

Compatible:

- adicionar endpoint;
- adicionar campo opcional;
- adicionar resposta/erro novo sem alterar os existentes.

Breaking (exige nova versão maior, como `/v2`):

- remover endpoint, operação ou campo obrigatório;
- tornar um campo obrigatório;
- mudar tipo ou semântica de um campo;
- remover valor de enum em uso.

Validação manual do comportamento:

```bash
# remover uma propriedade obrigatória deve falhar
API_BASE_FILE=/tmp/openapi-base.yaml API_REVISION_FILE=/tmp/openapi-breaking.yaml scripts/api-contract-check.sh

# adicionar uma propriedade opcional deve passar
API_BASE_FILE=/tmp/openapi-base.yaml API_REVISION_FILE=/tmp/openapi-compatible.yaml scripts/api-contract-check.sh
```

## Cliente TypeScript

O pacote `frontend/packages/api-client` é gerado a partir do contrato.

```bash
pnpm generate:api
pnpm --filter @mercado/api-client build
pnpm --filter @mercado/api-client typecheck
```

- `src/schema.ts` é gerado por `openapi-typescript` e versionado.
- `src/index.ts` adiciona um cliente `fetch` tipado, `ApiError` e métodos `get`/`post`/`patch`/`delete`.
- Requisições usam `credentials: "include"` para enviar o cookie de sessão.
- O build emite `dist/index.js`, `dist/index.d.ts` e os tipos do schema.

Uso:

```ts
import { ApiError, createClient } from "@mercado/api-client";

const api = createClient({ baseUrl: "http://127.0.0.1:8080" });

try {
  const page = await api.searchListings({ q: "fone", sort: "price_asc" });
  console.log(page.total);
  await api.login({ email: "buyer@example.com", password: "senha-de-teste" });
  console.log(await api.getMe());
} catch (error) {
  if (error instanceof ApiError) {
    console.error(error.code, error.correlationId);
  }
}
```

## Fluxo de alteração

1. Edite `api/openapi.yaml`.
2. Rode `pnpm lint:api`.
3. Gere o cliente com `pnpm generate:api`.
4. Rode `pnpm --filter @mercado/api-client build` e `typecheck`.
5. Atualize o backend e os consumidores.
6. Confirme que a mudança é compatível ou crie `/v2`.

O CI de pull request deve executar a validação e a checagem de breaking antes de aceitar a mudança. O workflow de release não altera o contrato.
