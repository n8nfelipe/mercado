# Spec Delta

## Purpose

Torna o comportamento da aplicação local diagnosticável por métricas, logs e traces correlacionados, sem expor dados pessoais ou segredos.

## ADDED Requirements

### Requirement: Logs estruturados

Os serviços SHALL emitir logs em JSON com timestamp, nível, serviço, correlation ID, rota, status e latência por requisição.

#### Scenario: Requisição registrada
- **WHEN** uma requisição HTTP é processada
- **THEN** um registro JSON é emitido com o correlation ID, a rota, o status e a duração da requisição

#### Scenario: Erro registrado
- **WHEN** uma requisição termina com erro
- **THEN** o log contém nível de erro e a causa, sem incluir stack trace não estruturado no fluxo de resposta

### Requirement: Correlation ID propagado

A API SHALL aceitar o cabeçalho `X-Correlation-ID` de entrada ou gerar um identificador quando ele estiver ausente, propagando-o para logs, traces e chamadas downstream.

#### Scenario: Correlation ID fornecido
- **WHEN** a requisição inclui `X-Correlation-ID`
- **THEN** o mesmo valor aparece na resposta, nos logs e nos spans da requisição

#### Scenario: Correlation ID ausente
- **WHEN** a requisição não inclui `X-Correlation-ID`
- **THEN** a API gera um identificador e o devolve na resposta e nos registros de observabilidade

### Requirement: Métricas RED por rota

A API SHALL expor métricas Prometheus de rate, errors e duration por rota e por status.

#### Scenario: Coleta de métricas
- **WHEN** o endpoint de métricas é consultado após tráfego
- **THEN** as séries de requisições, erros e latência da rota exercitada estão disponíveis

#### Scenario: Rota não exercitada
- **WHEN** uma rota ainda não recebeu tráfego
- **THEN** ela não produz série falsa de requisições

### Requirement: Traces distribuídos

Os serviços SHALL criar spans OpenTelemetry para requisições de entrada e para chamadas a PostgreSQL e Redis, exportando-os para o coletor local.

#### Scenario: Trace de uma requisição
- **WHEN** uma requisição acessa o banco
- **THEN** o trace contém o span HTTP e o span da operação de banco, com o mesmo trace ID

#### Scenario: Correlação entre log e trace
- **WHEN** uma requisição é rastreada
- **THEN** o log correspondente contém o trace ID do span de entrada

### Requirement: Dashboards provisionados

O ambiente SHALL provisionar dashboards e fontes de dados de observabilidade como código, sem configuração manual.

#### Scenario: Ambiente recém-criado
- **WHEN** o ambiente local termina de subir
- **THEN** os dashboards de latência, erros e saturação estão disponíveis no Grafana

#### Scenario: Recriação do ambiente
- **WHEN** o ambiente é destruído e recriado
- **THEN** os mesmos dashboards e fontes de dados são restaurados automaticamente

### Requirement: Proteção de dados sensíveis

Logs e traces SHALL redigir senhas, tokens, PAN e outros campos sensíveis antes da emissão.

#### Scenario: Requisição de login
- **WHEN** uma requisição contém senha ou token
- **THEN** os valores sensíveis não aparecem em logs nem em atributos de span

#### Scenario: Cabeçalho de autorização
- **WHEN** a requisição inclui `Authorization` ou cookie de sessão
- **THEN** o valor é mascarado nos registros de observabilidade
