# Spec Delta

## Purpose

Garante que o schema e os dados sintéticos possam ser criados, evoluídos, revertidos e reproduzidos de forma determinística no ambiente local e nos testes.

## ADDED Requirements

### Requirement: Schema criado do zero

O projeto SHALL aplicar todas as migrations pendentes em um banco vazio por um comando único até a versão mais recente do schema.

#### Scenario: Banco vazio
- **WHEN** o comando de migration é executado contra um banco vazio
- **THEN** todas as migrations são aplicadas em ordem e o schema fica na versão mais recente

#### Scenario: Migrations já aplicadas
- **WHEN** o comando de migration é executado em um banco já atualizado
- **THEN** nenhuma migration é reaplicada e o comando termina com sucesso

### Requirement: Migrations expand/contract

Toda migration SHALL seguir a estratégia expand/contract, mantendo compatibilidade com a versão anterior da aplicação durante a transição.

#### Scenario: Adição de coluna
- **WHEN** uma coluna é adicionada para uma nova funcionalidade
- **THEN** a adição é compatível com a versão anterior da aplicação e pode ser revertida antes da etapa de contrato

#### Scenario: Remoção de coluna
- **WHEN** uma coluna deixa de ser usada
- **THEN** a remoção ocorre em uma migration separada, somente após a aplicação parar de lê-la e escrevê-la

### Requirement: Rollback documentado e testado

Cada migration SHALL ter caminho de rollback ou correção forward documentado, e o rollback SHALL ser validado em ambiente local.

#### Scenario: Rollback da última versão
- **WHEN** o comando de rollback é executado a partir da versão mais recente
- **THEN** o schema retorna à versão anterior sem perda de dados incompatível com o rollback

#### Scenario: Migration sem down
- **WHEN** uma migration não possui rollback automático
- **THEN** o procedimento de correção forward está documentado e vinculado à migration

### Requirement: Seed determinístico

O seed SHALL gerar o mesmo conjunto de dados para a mesma semente e SHALL poder ser executado mais de uma vez sem duplicar registros.

#### Scenario: Duas execuções com a mesma semente
- **WHEN** o seed é executado duas vezes com a mesma configuração
- **THEN** as contagens de registros e os identificadores gerados são equivalentes, sem duplicatas

#### Scenario: Sementes diferentes
- **WHEN** o seed é executado com sementes diferentes
- **THEN** os conjuntos de dados gerados são distintos e reproduzíveis para cada semente

### Requirement: Migrations verificadas no CI

O pipeline SHALL aplicar as migrations contra um banco efêmero antes de aceitar um pull request que altere o schema.

#### Scenario: Migration inválida
- **WHEN** um pull request contém uma migration que falha ao aplicar em banco vazio
- **THEN** o pipeline falha e identifica a migration com erro

#### Scenario: Migration válida
- **WHEN** um pull request contém apenas migrations que aplicam e revertem corretamente
- **THEN** a verificação de schema passa
