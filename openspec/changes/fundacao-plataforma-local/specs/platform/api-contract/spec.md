# Spec Delta

## Purpose

Estabelece um contrato HTTP único, versionado e validado, que clientes e pipeline podem usar como fonte de verdade.

## ADDED Requirements

### Requirement: OpenAPI 3.1 como fonte de verdade

O projeto SHALL publicar o contrato da API como documento OpenAPI 3.1 versionado no repositório.

#### Scenario: Contrato disponível
- **WHEN** um cliente ou ferramenta precisa conhecer a API
- **THEN** ele obtém o documento OpenAPI 3.1 no caminho publicado pelo projeto

#### Scenario: Rota implementada
- **WHEN** uma rota HTTP é exposta pela API
- **THEN** ela está descrita no documento OpenAPI correspondente

### Requirement: Versionamento por caminho

A API SHALL versionar seus endpoints por segmento de versão maior no caminho, iniciando em `/v1`.

#### Scenario: Endpoint versionado
- **WHEN** um cliente chama um endpoint público
- **THEN** o caminho inclui a versão maior, como `/v1/...`

#### Scenario: Mudança incompatível
- **WHEN** uma alteração remove ou muda o significado de um contrato existente
- **THEN** ela é publicada em uma nova versão maior, preservando a anterior durante a migração

### Requirement: Erros padronizados

Todas as respostas de erro SHALL usar um envelope comum com código estável, mensagem legível, correlation ID e, quando aplicável, detalhes de validação por campo.

#### Scenario: Erro de validação
- **WHEN** uma requisição envia um campo inválido
- **THEN** a resposta contém código de erro, mensagem, correlation ID e a lista de campos inválidos

#### Scenario: Erro interno
- **WHEN** ocorre uma falha não esperada
- **THEN** a resposta contém código de erro interno e correlation ID, sem expor detalhes internos

### Requirement: Contrato validado no CI

O pipeline SHALL validar o documento OpenAPI e detectar mudanças incompatíveis antes de aceitar alterações de contrato.

#### Scenario: Documento inválido
- **WHEN** o documento OpenAPI contém erro de sintaxe ou referência quebrada
- **THEN** a validação falha e identifica o problema

#### Scenario: Mudança compatível
- **WHEN** um pull request adiciona um endpoint ou campo opcional
- **THEN** a validação passa e a versão maior é preservada
