-include .env
export

MIGRATIONS_DIR := app/migrations
GOOSE := GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DATABASE_URL)" GOOSE_MIGRATION_DIR=$(MIGRATIONS_DIR) goose

.PHONY: up down run test sqlc generate generate-ts lint-api migrate-new migrate-up migrate-down migrate-status

up:
	docker compose up -d --wait

down:
	docker compose down

run:
	go run ./app/cmd/server

test:
	go test ./...

sqlc:
	sqlc generate

OAPI_CODEGEN := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0

# Серверный код из api/openapi.yaml (ADR-0001). api.gen.go руками не править.
generate:
	mkdir -p app/internal/api
	go run $(OAPI_CODEGEN) -config api/oapi-codegen.yaml api/openapi.yaml

# TS-типы для фронта. web/ пока без package.json (T4), каталог вывода создаётся здесь.
generate-ts:
	mkdir -p ../web/src/api
	npx --yes openapi-typescript@7 api/openapi.yaml -o ../web/src/api/schema.d.ts

lint-api:
	npx --yes @redocly/cli@latest lint api/openapi.yaml

# make migrate-new name=create_runs
migrate-new:
	GOOSE_DRIVER=postgres goose -dir $(MIGRATIONS_DIR) create $(name) sql

migrate-up:
	$(GOOSE) up

migrate-down:
	$(GOOSE) down

migrate-status:
	$(GOOSE) status
