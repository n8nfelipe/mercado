SHELL := /usr/bin/env bash

.PHONY: dev-up dev-down migrate migrate-down migrate-status test smoke seed seed-catalog seed-bulk load lint-api

dev-up:
	./scripts/dev-up.sh

dev-down:
	./scripts/dev-down.sh

migrate:
	cd backend && go run ./cmd/migrate -direction up

migrate-down:
	cd backend && go run ./cmd/migrate -direction down

migrate-status:
	cd backend && go run ./cmd/migrate -direction status

test:
	cd backend && go test ./...
	pnpm -r --if-present typecheck
	pnpm -r --if-present test

smoke:
	./scripts/smoke.sh

seed:
	./scripts/seed.sh

seed-bulk:
	./scripts/seed-bulk.sh

seed-catalog:
	SEED_SELLER_EMAIL=$${SEED_SELLER_EMAIL:-vendedor@example.com} \
	SEED_SELLER_PASSWORD=$${SEED_SELLER_PASSWORD:-senha-super-segura} \
	SEED_SELLER_NAME=$${SEED_SELLER_NAME:-Loja do Bairro} \
	SEED_BUYER_EMAIL=$${SEED_BUYER_EMAIL:-comprador@example.com} \
	SEED_BUYER_PASSWORD=$${SEED_BUYER_PASSWORD:-outra-senha-segura} \
	./scripts/seed.sh

load:
	k6 run scripts/load.js

lint-api:
	./scripts/api-contract-check.sh
