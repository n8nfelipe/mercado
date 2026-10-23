# ADR-0001: Cluster local dedicado e portas do ingress

- **Status:** Aceito
- **Data:** 2026-10-22

## Contexto

O host já roda Docker, Umbrel e outros projetos. As portas 80 e 443 pertencem ao Umbrel, e existem clusters kind antigos (`raiz-comum` e `rancher`) que não devem ser reutilizados. O PRD exige um ambiente local reproduzível, isolado e descartável para o marketplace.

## Decisão

Criar um cluster kind dedicado chamado `mercado-dev`, inicialmente com um control-plane e workers adicionais apenas se houver recursos. O ingress-nginx usa o provider `kind` e publica as portas de host `8080` e `8443`.

## Consequências

- O cluster não é HA; a indisponibilidade do control-plane é uma limitação aceita do ambiente local.
- Os hostnames do projeto não conflitam com o Umbrel.
- O ambiente pode ser destruído e recriado sem afetar outros clusters, desde que os volumes de outros projetos não sejam tocados.
- A topologia é local e não é promovida para produção.

## Alternativas consideradas

- **k3d:** mais leve, mas menos alinhado ao kind já instalado e ao PRD.
- **minikube:** mais recursos e menos direto para múltiplos clusters kind existentes.
- **Reutilizar `raiz-comum`:** rejeitado por misturar blast radius e dados de outros projetos.

## Referências

- `PRD.md`, seções 12.1, 12.2 e 12.5.
- `openspec/changes/fundacao-plataforma-local/design.md`, decisões 1 e 2.
