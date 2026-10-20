SERVICES := ride-service auth-service user-service location-service payment-service

.PHONY: help
help:
	@echo "Usage: make [command] [service=<name>]"
	@echo ""
	@echo "  run service=ride-service            Run service"
	@echo "  dev service=ride-service            Hot reload service"
	@echo "  build service=ride-service          Build one service"
	@echo "  build-all                           Build all services"
	@echo "  migrate-up service=ride-service"
	@echo "  migrate-down service=ride-service"
	@echo "  migrate-status service=ride-service"
	@echo "  migrate-create service=ride-service name=<migration-name>"
	@echo "  migrate-reset service=ride-service"
	@echo "  docker-up / docker-down / docker-logs"

# Delegate to the service's own Makefile
SERVICE_TARGETS := run dev build migrate-up migrate-down migrate-status migrate-create migrate-reset

.PHONY: $(SERVICE_TARGETS)
$(SERVICE_TARGETS):
	@[ "$(service)" ] || (echo "Usage: make $@ service=<service-name>"; exit 1)
	@$(MAKE) -C $(service) $@ name=$(name)

.PHONY: build-all
build-all:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		$(MAKE) -C $$svc build || exit 1; \
	done

# Docker
COMPOSE_FILE := infra/development/docker/docker-compose.dev.yml

.PHONY: docker-up docker-down docker-logs
docker-up:
	@docker-compose -f $(COMPOSE_FILE) up -d

docker-down:
	@docker-compose -f $(COMPOSE_FILE) down

docker-logs:
	@docker-compose -f $(COMPOSE_FILE) logs -f