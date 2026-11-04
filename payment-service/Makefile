include .env
export

SERVICE_NAME=payment-service
MIGRATION_DIR=./sql/migrations

DB_DSN=postgres://$(PAYMENT_DB_USER):$(PAYMENT_DB_PASSWORD)@$(PAYMENT_DB_HOST):$(PAYMENT_DB_PORT)/$(PAYMENT_DB_NAME)?sslmode=disable

.PHONY: run dev build

run:
	@go run ./cmd/api/

dev:
	@air -c .air.toml

build:
	@go build -o ../../build/$(SERVICE_NAME) ./cmd/api/

# Migration (thay thế migrate.sh)
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

# Code quality
.PHONY: lint test tidy

lint:
	@golangci-lint run ./...

test:
	@go test -v -race ./...

tidy:
	@go mod tidy