# Ambiente local

## Topologia

```
Host
  portas 8080/8443
    |
    v
kind cluster mercado-dev (1 control-plane)
  ingress-nginx
    mercado.localhost        --> web:3000
    api.mercado.localhost    --> api:8080
    seller/admin (reservados)--> 404 ate o backoffice existir
  namespaces
    mercado             aplicacao
    mercado-data        PostgreSQL + Redis
    mercado-observability Prometheus + Grafana + Jaeger
```

## Pré-requisitos

- Docker
- kind
- kubectl
- Helm
- Go 1.26+
- Node.js 24+ e pnpm
- `ss` (iproute2) para o preflight de portas

## Comandos

```bash
make dev-up    # cria/atualiza o cluster e sobe o ambiente
make dev-down  # remove o cluster mercado-dev
```

O `make dev-up` é idempotente: se o cluster já existe, ele não é recriado. As portas 8080/8443 são verificadas antes da criação; se estiverem ocupadas, o script falha com mensagem explícita.

## Hostnames

- `http://mercado.localhost:8080` - frontend do comprador
- `http://api.mercado.localhost:8080` - API
- `http://grafana.mercado.localhost:8080` - Grafana (após o stack de observabilidade)
- `http://jaeger.mercado.localhost:8080` - Jaeger (após o stack de observabilidade)
- `seller.mercado.localhost` e `admin.mercado.localhost` ficam reservados e respondem 404 até o app `backoffice` existir.

## Governança de recursos

Cada namespace tem `ResourceQuota` e `LimitRange` com defaults explícitos. Um pod sem requests/limits recebe os valores padrão do namespace; um pod acima do máximo é rejeitado; a quota limita o agregado do namespace.

## Dados

O `make dev-up` instala o operador CloudNativePG e sobe PostgreSQL (1 instância) e Redis (StatefulSet com AOF) no namespace `mercado-data`. Ambos usam PVCs de 1Gi na StorageClass `local-path`; o kind fornece o provisioner por trás do alias. Detalhes, conexão e reset estão em `docs/local-data.md`.

## Limitações

- Um único control-plane: sem HA e sem multi-AZ.
- Disponibilidade best effort; não há SLA local.
- Sem DDoS de 15M RPS, sem 120M de usuários e sem PCI DSS.
- Dados sintéticos.
- `make dev-down` destrói o cluster e, com ele, os dados locais. A persistência entre reinícios de pod é garantida por PVC; a persistência entre `dev-down`/`dev-up` não é.

## Destruição

```bash
make dev-down
```

O comando remove apenas o cluster `mercado-dev`. Os clusters de outros projetos não são tocados.
