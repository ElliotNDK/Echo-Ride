
SERVICES=ride-service auth-service user-service location-service payment-service

.PHONY: help

help:
	@echo "Usage: make [command] [service=<name>]"
	@echo ""
	@echo "  run service=ride-service     Run service"
	@echo "  dev service=ride-service     Hot reload service"
	@echo "  build-all                    Build all services"
	@echo "  migrate-up service=ride-service"
	@echo "  docker-up / docker-down"

.PHONY: run dev build migrate-up migrate-down migrate-status migrate-create

run dev build migrate-up migrate-down migrate-status migrate-reset:
	@[ "$(service)" ] || (echo "Usage: make $@ service=<service-name>"; exit 1)
	@$(MAKE) -C services/$(service) $@ name=$(name)

.PHONY: build-all
build-all:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		$(MAKE) -C services/$$svc build; \
	done

# Docker
.PHONY: docker-up docker-down docker-logs
docker-up:
	@docker-compose -f infra/development/docker/docker-compose.dev.yml up -d

docker-down:
	@docker-compose -f infra/development/docker/docker-compose.dev.yml down

docker-logs:
	@docker-compose -f infra/development/docker/docker-compose.dev.yml logs -f
