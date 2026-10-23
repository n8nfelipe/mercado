# Spec Delta

## Purpose

Define um ambiente Kubernetes local dedicado, reproduzível e isolado, usado apenas para desenvolvimento, integração e testes de carga moderada do marketplace.

## ADDED Requirements

### Requirement: Cluster local dedicado

O ambiente SHALL prover um cluster kind dedicado chamado `mercado-dev`, sem reutilizar clusters de outros projetos.

#### Scenario: Criação a partir de host limpo
- **WHEN** o comando de subida é executado em um host sem o cluster
- **THEN** o cluster `mercado-dev` é criado e fica acessível pelo contexto kubectl do projeto

#### Scenario: Isolamento de outros clusters
- **WHEN** o ambiente é provisionado
- **THEN** nenhum workload do projeto é aplicado nos clusters `raiz-comum` ou `rancher`

### Requirement: Ciclo de vida reproduzível

O ambiente SHALL poder ser criado e destruído por comandos únicos e idempotentes, partindo de um host com Docker e kind instalados.

#### Scenario: Provisionamento completo
- **WHEN** o comando de subida é executado em um host limpo
- **THEN** cluster, dependências de dados e observabilidade ficam prontos em menos de 10 minutos

#### Scenario: Execução repetida
- **WHEN** o comando de subida é executado novamente em um ambiente já provisionado
- **THEN** o comando termina com sucesso sem duplicar recursos nem perder dados persistidos

#### Scenario: Destruição
- **WHEN** o comando de descida é executado
- **THEN** os recursos do projeto são removidos sem afetar outros clusters ou volumes do host

### Requirement: Ingress e hostnames locais

O ambiente SHALL expor os serviços por ingress-nginx nas portas de host `8080` e `8443`, usando hostnames `.localhost` estáveis.

#### Scenario: Roteamento da API
- **WHEN** uma requisição é feita para `api.mercado.localhost:8080`
- **THEN** ela é roteada para o serviço de API do projeto

#### Scenario: Roteamento do frontend do comprador
- **WHEN** uma requisição é feita para `mercado.localhost`
- **THEN** ela é roteada para o frontend do comprador

#### Scenario: Hostnames reservados sem frontend
- **WHEN** uma requisição é feita para `seller.mercado.localhost` ou `admin.mercado.localhost` antes de o frontend correspondente existir
- **THEN** o ingress responde com erro, sem encaminhar para um serviço não relacionado

#### Scenario: Frontend de seller ou admin publicado
- **WHEN** um frontend de seller ou admin é implantado
- **THEN** ele fica acessível no hostname reservado correspondente

#### Scenario: Host desconhecido
- **WHEN** uma requisição usa um hostname não declarado
- **THEN** o ingress responde com erro, sem encaminhar para um serviço do projeto

### Requirement: Separação por namespaces

O ambiente SHALL separar aplicação, dados e observabilidade nos namespaces `mercado`, `mercado-data` e `mercado-observability`.

#### Scenario: Workloads no namespace correto
- **WHEN** os manifestos do projeto são aplicados
- **THEN** cada workload é criado no namespace previsto para sua categoria

#### Scenario: Storage local
- **WHEN** um volume persistente é solicitado
- **THEN** ele é provisionado pela StorageClass `local-path` no namespace de dados

### Requirement: Governança de recursos

O ambiente SHALL aplicar `ResourceQuota` e `LimitRange` a cada namespace, de modo que todo pod resulte com requests e limits de CPU e memória, explícitos ou aplicados pelo `LimitRange`, dentro dos limites do namespace.

#### Scenario: Pod sem requests e limits
- **WHEN** um pod é submetido sem requests ou limits
- **THEN** o `LimitRange` aplica os valores padrão do namespace e o pod resultante respeita mínimo e máximo

#### Scenario: Pod acima do máximo
- **WHEN** um pod declara CPU ou memória acima do máximo do `LimitRange`
- **THEN** a admissão é rejeitada com mensagem que identifica o recurso e o limite

#### Scenario: Quota excedida
- **WHEN** um novo workload ultrapassa a quota do namespace
- **THEN** a criação é rejeitada e os workloads existentes continuam operando

### Requirement: Persistência fora do pod

O ambiente SHALL armazenar dados de estado em volumes persistentes, de modo que reinícios de pod não percam dados.

#### Scenario: Reinício do PostgreSQL
- **WHEN** o pod do PostgreSQL é reiniciado
- **THEN** os dados gravados antes do reinício continuam disponíveis após o pod voltar

### Requirement: Escopo estritamente local

O ambiente local SHALL operar com disponibilidade best effort e um único control-plane, sem representar HA, multi-AZ, SLA ou capacidade de produção.

#### Scenario: Topologia local
- **WHEN** o cluster é inspecionado
- **THEN** ele possui um único control-plane e nenhuma promessa de HA é configurada

#### Scenario: Falha de nó único
- **WHEN** o control-plane local fica indisponível
- **THEN** o ambiente pode ficar indisponível, e esse comportamento é aceito como limitação documentada do ambiente local
