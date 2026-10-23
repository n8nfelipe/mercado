# Deploy local

O deploy local usa kind, Kustomize, Tilt, ingress-nginx e os manifestos em
`deploy/k8s`. O fluxo principal é:

```bash
make dev-up
make smoke
make load
```

`make dev-up` é idempotente: cria/atualiza o cluster kind, instala ingress,
data plane e observabilidade, constrói as imagens locais, carrega no kind,
aplica o overlay e executa as migrations.

## Imagens e Kustomize

- `backend/Dockerfile` gera `mercado-api` com os binários `api`, `worker` e
  `migrate`.
- `mercado-worker` usa a mesma imagem com `command: ["/app/worker"]`.
- `mercado-web` é a imagem standalone do Next.js.
- `scripts/build-images.sh` constrói e executa `kind load docker-image`.
- `deploy/k8s/base` contém Deployments, Services e Ingress.
- `deploy/k8s/overlays/local` fixa a tag `local` e o namespace `mercado`.

Hosts locais:

| Host | Serviço |
| --- | --- |
| `mercado.localhost:8080` | frontend web |
| `api.mercado.localhost:8080` | API |
| `seller.mercado.localhost:8080` | reservado, responde 404 |
| `admin.mercado.localhost:8080` | reservado, responde 404 |
| `grafana.mercado.localhost:8080` | Grafana |
| `jaeger.mercado.localhost:8080` | Jaeger |

## Tilt

Com o cluster criado por `make dev-up`:

```bash
tilt up
```

O Tilt constrói as imagens de desenvolvimento, executa `kind load docker-image`
e aplica o overlay local. Portas de desenvolvimento: web em `3001` e API em
`18081`.

Live update:

- alterações em `backend/` recompilam o binário e reiniciam o container da API
  ou do worker;
- alterações em `frontend/` são sincronizadas no container web e o Next.js
  recarrega a página.

## Smoke e carga

```bash
make smoke
make load
```

`make smoke` verifica health/readiness da API, a resposta de `/v1/status`, a
página web e os hosts reservados. `make load` roda k6 com 1 VU por 5 segundos
contra `/v1/status`.

## Diagnóstico

```bash
kubectl get pods -n mercado
kubectl logs -n mercado deployment/api
kubectl logs -n mercado deployment/web
kubectl logs -n mercado deployment/worker
kubectl describe ingress -n mercado mercado
```

Para reconstruir após alterar manifests:

```bash
kubectl apply -k deploy/k8s/overlays/local
```
