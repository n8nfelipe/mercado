# Spec Delta

## Purpose

Define o comportamento operacional que todo serviço deve expor para ser executado, reiniciado e protegido de forma previsível no Kubernetes.

## ADDED Requirements

### Requirement: Liveness e readiness distintos

Cada serviço SHALL expor `/healthz` para liveness, sem depender de dependências externas, e `/readyz` para readiness, que reflete a disponibilidade das dependências obrigatórias.

#### Scenario: Dependência indisponível
- **WHEN** o PostgreSQL está fora do ar
- **THEN** `/healthz` responde com sucesso e `/readyz` responde indicando indisponibilidade

#### Scenario: Dependências disponíveis
- **WHEN** as dependências obrigatórias estão acessíveis
- **THEN** `/readyz` responde com sucesso

### Requirement: Encerramento gracioso

Ao receber `SIGTERM`, o serviço SHALL parar de aceitar novas requisições, concluir as requisições em andamento dentro de um timeout configurado e liberar os recursos antes de encerrar.

#### Scenario: Encerramento com requisição em andamento
- **WHEN** o pod recebe `SIGTERM` durante uma requisição
- **THEN** a requisição em andamento é concluída e novas conexões são recusadas

#### Scenario: Timeout de drenagem
- **WHEN** uma requisição ultrapassa o timeout de drenagem
- **THEN** o serviço encerra após o timeout e registra a requisição interrompida

### Requirement: Rate limiting consistente

A API SHALL aplicar limites por IP e API key e, quando houver autenticação, também por usuário, compartilhados entre réplicas, respondendo com status `429` e `Retry-After` ao exceder o limite.

#### Scenario: Limite excedido
- **WHEN** um cliente ultrapassa o limite configurado para sua chave
- **THEN** a API responde `429` com `Retry-After` e não executa a operação

#### Scenario: Chaves isoladas
- **WHEN** um cliente atinge o limite e outro cliente usa uma chave diferente
- **THEN** o segundo cliente continua sendo atendido dentro do próprio limite

#### Scenario: Réplicas múltiplas
- **WHEN** a API opera com mais de uma réplica
- **THEN** o limite é contabilizado de forma compartilhada, não por réplica

### Requirement: Recuperação de panic

A API SHALL capturar panics em handlers, responder com erro interno, registrar o evento e continuar atendendo novas requisições.

#### Scenario: Handler com panic
- **WHEN** um handler entra em panic durante uma requisição
- **THEN** o cliente recebe uma resposta de erro interno e o processo permanece ativo

#### Scenario: Requisição seguinte
- **WHEN** uma nova requisição chega após um panic recuperado
- **THEN** ela é atendida normalmente

### Requirement: Limites de requisição

A API SHALL impor tamanho máximo de corpo e timeouts de leitura, escrita e idle configuráveis por ambiente.

#### Scenario: Corpo acima do limite
- **WHEN** uma requisição excede o tamanho máximo configurado
- **THEN** a API rejeita a requisição sem processar o corpo completo

#### Scenario: Cliente lento
- **WHEN** um cliente não envia a requisição dentro do timeout de leitura
- **THEN** a conexão é encerrada e o evento é registrado

### Requirement: Configuração validada na inicialização

O serviço SHALL validar a configuração obrigatória na inicialização e SHALL falhar rápido, com mensagem explícita, quando ela estiver ausente ou inválida.

#### Scenario: Configuração incompleta
- **WHEN** o serviço inicia sem uma variável obrigatória
- **THEN** ele encerra com erro que identifica a variável ausente

#### Scenario: Configuração válida
- **WHEN** todas as variáveis obrigatórias estão presentes e válidas
- **THEN** o serviço inicia e fica pronto para receber tráfego
