# Makefile (place at project root)

# Default Compose files
DEV_COMPOSE = docker-compose.dev.yml
PROD_COMPOSE = docker-compose.yml

## dev-up: Create dev stack with live reload
dev-up:
	@echo "Creating dev stack..."
	docker compose -f $(DEV_COMPOSE) up --build -d

## dev-up-scaled: Create dev stack with scaled workers (default WORKERS=3)
WORKERS ?= 3
dev-up-scaled:
	@echo "Creating dev stack with $(WORKERS) workers ..."
	docker compose -f $(DEV_COMPOSE) up --build -d --scale worker=$(WORKERS)

## dev-down: Stop dev stack
dev-down:
	@echo "Stopping dev stack..."
	docker compose -f $(DEV_COMPOSE) down

## dev-stop: Start dev stack
dev-start:
	@echo "Start dev stack..."
	docker compose -f $(DEV_COMPOSE) start

## dev-stop: Halt dev stack
dev-stop:
	@echo "Halting dev stack..."
	docker compose -f $(DEV_COMPOSE) stop

## prod-up: Create production-like stack
prod-up:
	@echo "Creating prod stack..."
	docker compose -f $(PROD_COMPOSE) up --build -d

## prod-down: Stop production-like stack
prod-down:
	@echo "Stopping prod stack..."
	docker compose -f $(PROD_COMPOSE) down

## dev-stop: Halt dev stack
prod-stop:
	@echo "Halting prod stack..."
	docker compose -f $(PROD_COMPOSE) stop

## logs: Tail logs for all services
logs:
	@echo "Tailing logs..."
	docker compose -f $(DEV_COMPOSE) logs -f

## restart: Restart dev stack
restart: dev-down dev-up
	@echo "Dev stack restarted."
