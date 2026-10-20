include .env
export

SERVICE_NAME=user-service
MIGRATION_DIR=./sql/migrations

DB_DSN=postgres://$(USER_DB_USER):$(USER_DB_PASSWORD)@$(USER_DB_HOST):$(USER_DB_PORT)/$(USER_DB_NAME)?sslmode=disable

.PHONY: run dev build

run:
	@go run ./cmd/api/

dev:
	@air -c .air.toml

build:
	@go build -o ../../build/$(SERVICE_NAME) ./cmd/api/

.PHONY: migrate-create migrate-up migrate-down migrate-status migrate-reset

migrate-create:
	@[ "$(name)" ] || (echo "Usage: make migrate-create name=<migration_name>"; exit 1)
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DB_DSN)" \
		goose -dir $(MIGRATION_DIR) create $(name) sql

migrate-up:
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DB_DSN)" \
		goose -dir $(MIGRATION_DIR) up

migrate-down:
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DB_DSN)" \
		goose -dir $(MIGRATION_DIR) down

migrate-status:
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DB_DSN)" \
		goose -dir $(MIGRATION_DIR) status

migrate-reset:
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DB_DSN)" \
		goose -dir $(MIGRATION_DIR) down-to 0
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DB_DSN)" \
		goose -dir $(MIGRATION_DIR) up

.PHONY: lint test tidy

lint:
	@golangci-lint run ./...

test:
	@go test -v -race ./...

tidy:
	@go mod tidy
