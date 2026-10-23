# ADR-0003: Regra de CI do projeto prevalece sobre o PRD

- **Status:** Aceito
- **Data:** 2026-10-22

## Contexto

O PRD (seção 15.1) descreve que, no merge para `main`, o pipeline faria build imutável, assinatura com cosign, push no registry e criação da tag. A regra de CI do projeto, carregada pela skill `regra-ci`, determina outro contrato: pull request roda somente verificações; merge na `main` roda somente a criação da tag de release.

## Decisão

Seguir a regra do projeto. `ci.yml` roda apenas em `pull_request` e cobre testes, lint, typecheck e análise estática, além de OpenAPI, build, SBOM e scan. `release.yml` roda apenas em push para `main`, com `contents: write`, e cria uma tag semver anotada, começando em `v1.0.0`.

Build, assinatura e publicação de imagens não acontecem no merge. Eles ficam para a change de prontidão para produção, quando o fluxo de release for definido.

## Consequências

- O merge é rápido e não repete verificações já aprovadas no PR.
- A rastreabilidade da release fica na tag semver; a publicação de imagem é uma etapa separada e futura.
- O PRD precisa ser lido com esta exceção explícita; a divergência está documentada em `docs/ci.md`.
- Um pull request com alteração incompatível de contrato continua bloqueado pelo job de contrato.

## Alternativas consideradas

- **Seguir o PRD literalmente:** violaria a regra de CI do projeto.
- **`semantic-release`:** automatiza tag e publicação, mas mistura responsabilidades e publica artefatos no merge.
- **Rodar checks também no merge:** redundante e proibido pela regra.

## Referências

- Skill `regra-ci`.
- `PRD.md`, seção 15.1.
- `openspec/changes/fundacao-plataforma-local/design.md`, decisão 12.
