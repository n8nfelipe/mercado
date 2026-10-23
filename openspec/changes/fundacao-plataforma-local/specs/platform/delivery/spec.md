# Spec Delta

## Purpose

Define como o projeto é validado em pull requests e como releases rastreáveis são criadas no merge para a branch principal.

## ADDED Requirements

### Requirement: Pull request apenas valida

Em pull request, o pipeline SHALL executar lint, testes, build, validação do OpenAPI e scans de dependências e de container, sem criar tag de release nem publicar artefato de release.

#### Scenario: Pull request aprovado
- **WHEN** um pull request é aberto ou atualizado
- **THEN** todas as verificações são executadas e o resultado de cada uma é reportado no pull request

#### Scenario: Verificação falha
- **WHEN** qualquer verificação falha
- **THEN** o pipeline falha e nenhuma tag de release é criada

#### Scenario: Nenhuma tag em pull request
- **WHEN** o pipeline de pull request termina com sucesso
- **THEN** nenhuma tag de release é criada e nenhuma imagem é promovida

### Requirement: Merge na main cria a tag de release

No merge para `main`, o pipeline SHALL criar exatamente uma tag de release a partir do commit integrado, sem repetir a suíte de validação.

#### Scenario: Merge bem-sucedido
- **WHEN** um commit é integrado na `main`
- **THEN** uma única tag de release é criada para aquele commit

#### Scenario: Falha na criação do artefato
- **WHEN** o build do artefato falha após o merge
- **THEN** nenhuma tag de release é criada e o pipeline reporta a falha

#### Scenario: Reprocessamento
- **WHEN** o pipeline de release é executado novamente para o mesmo commit
- **THEN** ele não cria uma segunda tag para o mesmo commit

### Requirement: Build de imagem validado no PR

O pipeline de pull request SHALL construir a imagem do serviço, gerar SBOM e executar scan de vulnerabilidades sobre ela, sem publicar ou promover a imagem.

#### Scenario: Build com SBOM
- **WHEN** um pull request altera o backend ou o frontend
- **THEN** a imagem é construída e um SBOM correspondente é gerado para inspeção

#### Scenario: Vulnerabilidade crítica
- **WHEN** o scan encontra uma vulnerabilidade crítica conhecida na imagem
- **THEN** o pipeline falha e o pull request é bloqueado

#### Scenario: Imagem efêmera
- **WHEN** o pipeline de pull request termina
- **THEN** nenhuma imagem é publicada em registry de release nem promovida a ambiente

### Requirement: Validação do contrato OpenAPI no CI

O pipeline SHALL validar sintaxe e compatibilidade do documento OpenAPI em todo pull request que altere o contrato.

#### Scenario: Alteração compatível
- **WHEN** um pull request adiciona um endpoint ou campo opcional ao contrato
- **THEN** a validação de contrato passa

#### Scenario: Alteração incompatível
- **WHEN** um pull request remove ou renomeia um campo ou endpoint existente sem nova versão maior
- **THEN** a validação de contrato falha e o pull request é bloqueado
