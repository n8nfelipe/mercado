# ADR-0002: Data plane local com CloudNativePG e Redis

- **Status:** Aceito
- **Data:** 2026-10-22

## Contexto

O orçamento local é de aproximadamente 7 GB de RAM para todo o ambiente, com cerca de 9,5 GB disponíveis no host. O PRD pede PostgreSQL via CloudNativePG e Redis local, mas também pede que Meilisearch, MinIO, Redpanda e Mailpit entrem apenas quando necessários.

## Decisão

Usar CloudNativePG com uma instância de PostgreSQL e Redis como StatefulSet de um pod. Instalar Meilisearch, MinIO, Redpanda e Mailpit apenas nas changes que os consomem. Se o operador do CloudNativePG comprometer o orçamento, o fallback é PostgreSQL em StatefulSet puro, registrado em nova ADR.

## Consequências

- O caminho local fica próximo do stack de produção e ganha backups/gestão de cluster sem custo imediato de HA.
- O operador adiciona CRDs e um pod extra; o consumo deve ser medido no primeiro `make dev-up`.
- O Redis é efêmero do ponto de vista de HA: um único pod, com PVC para persistência local.
- A troca por serviços gerenciados em produção continua sendo feita por configuração, não por mudança de código de aplicação.

## Alternativas consideradas

- **PostgreSQL em StatefulSet puro:** mais simples e leve, porém perde o alinhamento com o PRD e os backups gerenciados.
- **Serviços gerenciados locais:** não existem no host.
- **Instalar todos os stores agora:** rejeitado por estourar RAM e aumentar o tempo de subida.

## Referências

- `PRD.md`, seções 11.2, 12.3 e 12.4.
- `openspec/changes/fundacao-plataforma-local/design.md`, decisões 3 e 4.
