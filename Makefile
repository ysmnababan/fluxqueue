# Makefile (place at project root)

# Default Compose files
DEV_COMPOSE = docker-compose.dev.yml
PROD_COMPOSE = docker-compose.yml

## dev-up: Start dev stack with live reload
dev-up:
	@echo "Starting dev stack..."
	docker compose -f $(DEV_COMPOSE) up --build -d

## dev-down: Stop dev stack
dev-down:
	@echo "Stopping dev stack..."
	docker compose -f $(DEV_COMPOSE) down

## prod-up: Start production-like stack
prod-up:
	@echo "Starting prod stack..."
	docker compose -f $(PROD_COMPOSE) up --build -d

## prod-down: Stop production-like stack
prod-down:
	@echo "Stopping prod stack..."
	docker compose -f $(PROD_COMPOSE) down

## logs: Tail logs for all services
logs:
	@echo "Tailing logs..."
	docker compose -f $(DEV_COMPOSE) logs -f

## restart: Restart dev stack
restart: dev-down dev-up
	@echo "Dev stack restarted."
