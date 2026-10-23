# Migrations e seed

## Estrutura

- `backend/migrations/*.sql`: migrations SQL com anotações `-- +goose Up` / `-- +goose Down`.
- `backend/migrations/embed.go`: embute os SQLs no binário.
- `backend/cmd/migrate`: aplica, reverte, reseta ou mostra o status.
- `backend/cmd/seed`: popula os dados de referência de forma determinística.
- `backend/internal/reference`: catálogo, seleção determinística, `Seed` e `SeedCatalog`.
- `scripts/seed.sh`: aplica migrations e roda o seed no ambiente local.

O schema inicial contém `currencies`, `countries` e `locales`. A migration `00002` adiciona `locales.is_extra`, usada para distinguir locales de referência dos locales extras escolhidos pela semente.

## Comandos

### Ambiente local

```bash
make seed          # cria apenas as categorias de catálogo
SEED=1 make seed
SEED=2 make seed
make seed-catalog  # cria categorias, contas de exemplo e anúncios publicados
```

Sem `DATABASE_URL`, o script abre um `kubectl port-forward` para `postgres-rw`, lê a URI do secret `postgres-app` e usa `127.0.0.1:15432`. Para outra porta:

```bash
SEED_DB_PORT=15433 make seed
```

`make seed` sempre cria as categorias de catálogo de forma idempotente. Para criar também as contas de exemplo e os anúncios publicados, defina as variáveis `SEED_SELLER_EMAIL`/`SEED_SELLER_PASSWORD` (obrigatórias em par) e, opcionalmente, `SEED_BUYER_EMAIL`/`SEED_BUYER_PASSWORD`; `make seed-catalog` fornece valores padrão. As senhas nunca são impressas nem gravadas em texto claro — apenas o hash Argon2id.

O port-forward usa `sslmode=disable` porque o túnel local termina a conexão TLS no host e o reset de TLS encerra o túnel do `kubectl`.

### Com uma URI explícita

```bash
cd backend
DATABASE_URL='postgres://...' go run ./cmd/migrate -direction up
DATABASE_URL='postgres://...' go run ./cmd/seed -seed 1
```

O `Makefile` expõe os mesmos comandos:

```bash
DATABASE_URL='postgres://...' make migrate
DATABASE_URL='postgres://...' make migrate-down
DATABASE_URL='postgres://...' make migrate-status
```

## Determinismo do seed

- A mesma semente produz o mesmo conjunto de países, moedas e locales e o mesmo `fingerprint`.
- Repetir a mesma semente faz upsert e remove extras obsoletos, sem duplicar linhas.
- Sementes diferentes escolhem um subconjunto diferente de locales extras; países e moedas canônicos permanecem iguais.
- `SEED` também pode ser definido por variável de ambiente.
- Os IDs são os próprios códigos (`BR`, `BRL`, `pt-BR`), portanto são estáveis entre execuções.

Exemplo:

```text
seed=1 currencies=12 countries=13 locales=16 fingerprint=10a68459...
seed=2 currencies=12 countries=13 locales=17 fingerprint=ba79674e...
```

### Seed de catálogo

```bash
cd backend
DATABASE_URL='postgres://...' \
  SEED_SELLER_EMAIL=vendedor@example.com \
  SEED_SELLER_PASSWORD=senha-super-segura \
  SEED_SELLER_NAME='Loja do Bairro' \
  SEED_BUYER_EMAIL=comprador@example.com \
  SEED_BUYER_PASSWORD=outra-senha-segura \
  go run ./cmd/seed -seed 1
```

- Categorias, contas, perfis de vendedor e anúncios usam UUIDs derivados (SHA-1 sobre um namespace fixo), então reexecuções fazem `ON CONFLICT DO NOTHING` e não duplicam registros.
- Sem nenhuma variável `SEED_*` de conta, o comando cria somente categorias.
- Os quatro anúncios de exemplo nascem com status `published` e estoque inicial (um deles com estoque zero, para exercitar a indisponibilidade na vitrine).
- Definir uma senha sem o e-mail correspondente falha com mensagem explícita.

### Carga em volume

```bash
make seed-bulk                      # 100.000 anúncios publicados
COUNT=5000 make seed-bulk           # quantidade customizada
SEED_SELLER_EMAIL=outra@example.com make seed-bulk
```

`seed-bulk` usa `backend/cmd/seedbulk` para inserir anúncios publicados em lotes (padrão 1000 linhas por `INSERT ... unnest ... ON CONFLICT DO NOTHING`) no vendedor informado, distribuindo entre as categorias ativas e criando o estoque inicial. Os UUIDs são determinísticos por índice, então reexecutar não duplica registros (os já existentes são ignorados). Exige que as categorias e o vendedor existam — rode `make seed-catalog` antes. Em ~100.000 linhas o tempo observado foi de cerca de 28 s.

## Reset e rollback

### Reverter a última migration

```bash
DATABASE_URL='postgres://...' make migrate-down
```

Isso executa o `Down` da migration mais recente e preserva os dados compatíveis. No caso da `00002`, a coluna `is_extra` é removida e as linhas de `locales` permanecem; ao reaplicar com `make migrate`, o seed volta a marcar e limpar os extras conhecidos.

### Reset completo do schema

```bash
cd backend
DATABASE_URL='postgres://...' go run ./cmd/migrate -direction reset
DATABASE_URL='postgres://...' go run ./cmd/migrate -direction up
DATABASE_URL='postgres://...' go run ./cmd/seed -seed 1
```

`reset` executa todos os `Down` na ordem inversa e remove o schema gerenciado. Para apagar também o cluster e os volumes locais, use `make dev-down` e depois `make dev-up`.

### Migration sem `Down`

Se uma migration não puder ser revertida com segurança, o `Down` deve ser omitido ou limitado, e o arquivo deve apontar para o procedimento de correção forward. O procedimento precisa informar o comando, a versão de destino e como validar os dados antes e depois. Nunca remova uma coluna usada pela versão anterior na mesma migration que introduz a substituição: siga expand/contract.

## Expand/contract

1. **Expand:** adicione colunas/tabelas compatíveis, faça backfill e mantenha leitura e escrita antigas funcionando.
2. **Migrate:** a aplicação passa a usar o novo schema.
3. **Contract:** remova o schema antigo em uma migration separada, depois que nenhuma versão em execução o utiliza.

Adições são reversíveis com `Down`; remoções só entram depois da etapa de contrato.

## Teste de integração

Os testes em `backend/internal/reference/seed_test.go` e `backend/internal/reference/catalog_seed_test.go` usam Testcontainers com `postgres:16-alpine` e verificam:

- aplicação das migrations em banco vazio;
- seed repetido com a mesma semente sem duplicatas;
- sementes diferentes gerando locales diferentes;
- rollback da última migration preservando as linhas;
- reaplicação e seed restaurando o mesmo conjunto;
- reset removendo as tabelas;
- seed de catálogo idempotente (duas execuções não duplicam categorias, contas ou anúncios);
- seed de catálogo sem variáveis criando apenas categorias;
- senhas de exemplo armazenadas apenas como hash Argon2id.

```bash
cd backend
go test ./internal/reference -v
```

O teste é ignorado quando Docker não está disponível. No CI, o runner precisa de Docker para executar essa verificação.

## CI

O workflow de pull request deve rodar `go test ./...` com Docker disponível; a falha de uma migration inválida interrompe o pipeline. O workflow de release não executa migrations nem seed.
