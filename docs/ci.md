# CI e release

## Regra do projeto

O projeto segue a regra de CI da skill `regra-ci`:

| Evento | Workflow | O que roda |
|---|---|---|
| Abrir ou atualizar pull request | `.github/workflows/ci.yml` | somente verificações |
| Merge (push) na `main` | `.github/workflows/release.yml` | somente criação da tag de release |

`ci.yml` usa `on: pull_request` e `permissions: contents: read`. `release.yml` usa `on: push: branches: [main]` e `permissions: contents: write`. Nenhum dos dois mistura as responsabilidades.

## Verificações do pull request

O contrato mínimo é composto por quatro categorias:

1. **Testes:** `go test ./...` e `pnpm -r --if-present test`.
2. **Lint:** `golangci-lint run`; o frontend usa `pnpm -r --if-present lint` e hoje não define um linter próprio.
3. **Typecheck:** `go vet ./...` para Go e `pnpm -r --if-present typecheck` para TypeScript.
4. **Análise estática:** `golangci-lint` (regras estáticas) e `tsc --noEmit` no frontend.

Além dessas categorias, o PR valida o contrato OpenAPI com `./scripts/api-contract-check.sh`, constrói as imagens de API e web, gera SBOM com Syft e executa scan de vulnerabilidades críticas com Trivy. A imagem construída no PR é efêmera: não é publicada nem promovida.

## Release

No merge para `main`, `release.yml` calcula o próximo semver a partir dos commits desde a última tag:

- breaking change -> major;
- funcionalidade nova -> minor;
- correção ou outro -> patch.

A primeira tag é `v1.0.0`. Se o HEAD já estiver marcado, o workflow não cria outra tag. A tag é anotada e enviada pelo bot da action.

## Divergência deliberada com o PRD

O PRD, seção 15.1, prevê build imutável, assinatura com cosign e push no registry no merge. A regra de CI do projeto prevalece: o merge só cria a tag. Build, assinatura e publicação de imagens ficam para a change de prontidão para produção. Ver `docs/adr/0003-regra-de-ci-prevalece.md`.

## Pré-requisito: remote GitHub

O repositório local ainda não tem remote GitHub definido. Os workflows podem ser versionados, mas só executam depois que o owner, o nome do repositório e o remote forem configurados. Enquanto isso, a validação local é feita com `actionlint` e com o teste da lógica de versão em um repositório sintético.

## Validação local

```bash
actionlint .github/workflows/*.yml
./scripts/next-version.sh --self-test
./scripts/api-contract-check.sh
openspec validate fundacao-plataforma-local --strict
```
