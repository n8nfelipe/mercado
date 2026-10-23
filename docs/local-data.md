# Data plane local

## Componentes

| Componente | Implementação local | Namespace |
|---|---|---|
| PostgreSQL | CloudNativePG, 1 instância, 1Gi | `mercado-data` |
| Redis | StatefulSet de 1 pod, AOF, 1Gi | `mercado-data` |
| MinIO | Deployment de 1 pod, PVC de 5Gi, bucket `mercado-listings` | `mercado-data` |

O PostgreSQL é criado pelo recurso `Cluster/postgres`. O operador do CloudNativePG gera o secret `postgres-app` com a URI de conexão. O Redis e o MinIO usam credenciais do secret `mercado-secrets`. O MinIO é aplicado por `deploy/k8s/data` e o bucket é criado pelo job `deploy/k8s/base/jobs/minio-init.yaml`.

O kind já fornece a StorageClass `standard` com o provisioner `rancher.io/local-path`. O arquivo `deploy/k8s/base/storageclass.yaml` cria o alias `local-path`, usado pelos manifests, e é aplicado pelo `make dev-up`.

## Conexão

```bash
# PostgreSQL, administração local via socket (peer auth)
kubectl exec -n mercado-data postgres-1 -- psql -U postgres -d mercado -c 'select 1'

# PostgreSQL, usuário da aplicação via TCP
kubectl exec -n mercado-data postgres-1 -- env PGPASSWORD="$(kubectl get secret postgres-app -n mercado-data -o jsonpath='{.data.password}' | base64 -d)" psql -h postgres-rw -U mercado -d mercado -c 'select current_user'

# Redis
kubectl exec -n mercado-data redis-0 -- sh -c 'redis-cli -a "$REDIS_PASSWORD" ping'

# MinIO via port-forward (API S3 em 9000, console em 9001)
kubectl port-forward -n mercado-data svc/minio 15434:9000 15435:9001

# Conferir o bucket pelo job de inicialização
kubectl logs -n mercado-data job/minio-init
```

O usuário `mercado` não autentica por peer no socket local; use TCP (`-h postgres-rw`) e a senha do secret. A API recebe `DATABASE_URL` e `REDIS_PASSWORD` do secret `mercado-secrets` no namespace `mercado`. O Redis recebe a senha do mesmo secret no namespace `mercado-data`.

## Seed

```bash
make seed          # apenas categorias de catálogo
make seed-catalog  # categorias, contas de exemplo e anúncios publicados
```

`make seed-catalog` usa `SEED_SELLER_*`/`SEED_BUYER_*` (com valores padrão) e é idempotente: reexecutar não duplica registros. As senhas nunca são impressas nem versionadas.

## Segredos

```bash
./scripts/dev-secrets.sh
```

O script espera o secret `postgres-app`, lê a URI do PostgreSQL e gera, na primeira execução, as credenciais de Redis e MinIO (`minio-access-key` e `minio-secret-key`). Ele aplica `mercado-secrets` nos namespaces `mercado` e `mercado-data`. Nenhum valor é impresso ou versionado. Para rotacionar as credenciais, remova o secret e execute o script novamente; o job `minio-init` recria o bucket.

## Persistência

- PostgreSQL: PVC gerenciado pelo CloudNativePG com StorageClass `local-path`.
- Redis: `volumeClaimTemplate` com StorageClass `local-path` e AOF habilitado.
- MinIO: PVC `minio-data` com StorageClass `local-path`. As imagens ficam em `mercado-listings` com object key `listings/<listing_id>/<image_id>.<ext>`.

Reiniciar um pod preserva os dados. `make dev-down` destrói o cluster kind e, portanto, os dados locais.

## Reset

```bash
kubectl delete cluster postgres -n mercado-data
kubectl delete statefulset redis -n mercado-data
kubectl delete deployment minio -n mercado-data
kubectl delete pvc --all -n mercado-data
kubectl delete secret postgres-app -n mercado-data --ignore-not-found
make dev-up
```

O reset remove os dados e recria PostgreSQL, Redis e MinIO do zero. O secret `mercado-secrets` é preservado; para rotacionar também as credenciais, remova-o antes do `make dev-up`. Se o deployment `minio` já não existir, ignore o erro do `delete`.

## Fallback

Se o operador do CloudNativePG consumir RAM demais ou falhar no kind, o fallback documentado é trocar `postgres-cluster.yaml` por um StatefulSet de PostgreSQL com um PVC `local-path`. A aplicação continua usando a mesma `DATABASE_URL`; apenas o provisionamento muda.
