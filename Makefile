# ── Hardware Store API Makefile ──────────────────────────────────────────────
# Usage: make <command>
# Example: make up, make logs, make migrate

# ── Variables ──────────────────────────────────────────────────────────────────
DOCKER_COMPOSE = docker-compose
DOCKER = docker
GO = go
MIGRATIONS_PATH = ./migrations
API_URL = http://localhost:8080
API_VERSION = /api/v1

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RED    := $(shell tput -Txterm setaf 1)
BLUE   := $(shell tput -Txterm setaf 4)
RESET  := $(shell tput -Txterm sgr0)

# ── Help ──────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help message
	@echo ''
	@echo '${GREEN}Hardware Store API${RESET} - Makefile Commands'
	@echo ''
	@echo '${BLUE}Docker Commands:${RESET}'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "${GREEN}%-20s${RESET} %s\n", $$1, $$2}'
	@echo ''

# ── Docker Commands ──────────────────────────────────────────────────────────
.PHONY: up
up: ## Start all services in detached mode
	@echo "${GREEN}Starting all services...${RESET}"
	$(DOCKER_COMPOSE) up -d
	@echo "${GREEN}Services started!${RESET}"
	@make ps

.PHONY: down
down: ## Stop all services
	@echo "${YELLOW}Stopping all services...${RESET}"
	$(DOCKER_COMPOSE) down
	@echo "${GREEN}Services stopped!${RESET}"

.PHONY: restart
restart: ## Restart all services
	@echo "${YELLOW}Restarting all services...${RESET}"
	$(DOCKER_COMPOSE) restart
	@echo "${GREEN}Services restarted!${RESET}"
	@make ps

.PHONY: logs
logs: ## Show logs for all services (use: make logs api, make logs postgres)
	@if [ -n "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "${GREEN}Showing logs for: $(filter-out $@,$(MAKECMDGOALS))${RESET}"; \
		$(DOCKER_COMPOSE) logs -f $(filter-out $@,$(MAKECMDGOALS)); \
	else \
		echo "${GREEN}Showing logs for all services...${RESET}"; \
		$(DOCKER_COMPOSE) logs -f; \
	fi

.PHONY: ps
ps: ## Show running containers status
	@echo "${GREEN}Container Status:${RESET}"
	$(DOCKER_COMPOSE) ps

.PHONY: build
build: ## Rebuild the API container
	@echo "${YELLOW}Building API container...${RESET}"
	$(DOCKER_COMPOSE) build --no-cache api
	@echo "${GREEN}Build complete!${RESET}"

.PHONY: rebuild
rebuild: down build up ## Rebuild and restart all services
	@echo "${GREEN}Rebuild complete!${RESET}"

# ── API Commands ──────────────────────────────────────────────────────────────
.PHONY: health
health: ## Check API health
	@echo "${GREEN}Checking API health...${RESET}"
	@curl -s $(API_URL)$(API_VERSION)/health | jq '.' 2>/dev/null || curl -s $(API_URL)$(API_VERSION)/health
	@echo ""

.PHONY: api-status
api-status: ## Show API status
	@echo "${GREEN}API Status:${RESET}"
	@curl -s $(API_URL)$(API_VERSION)/health | jq '.' 2>/dev/null || echo "${RED}API not responding${RESET}"

.PHONY: api-products
api-products: ## Get products list
	@echo "${GREEN}Fetching products...${RESET}"
	@curl -s $(API_URL)$(API_VERSION)/products | jq '.' 2>/dev/null || curl -s $(API_URL)$(API_VERSION)/products

.PHONY: api-stores
api-stores: ## Get stores list
	@echo "${GREEN}Fetching stores...${RESET}"
	@curl -s $(API_URL)$(API_VERSION)/stores | jq '.' 2>/dev/null || curl -s $(API_URL)$(API_VERSION)/stores

.PHONY: api-categories
api-categories: ## Get categories list
	@echo "${GREEN}Fetching categories...${RESET}"
	@curl -s $(API_URL)$(API_VERSION)/categories | jq '.' 2>/dev/null || curl -s $(API_URL)$(API_VERSION)/categories

# ── Database Commands ─────────────────────────────────────────────────────────
.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	@echo "${GREEN}Connecting to PostgreSQL...${RESET}"
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -d hardware_store

.PHONY: db-reset
db-reset: ## Reset database (WARNING: deletes all data!)
	@echo "${RED}WARNING: This will delete ALL database data!${RESET}"
	@read -p "Are you sure? (y/N) " -n 1 -r; \
	echo ""; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "${YELLOW}Resetting database...${RESET}"; \
		$(DOCKER_COMPOSE) down -v; \
		$(DOCKER_COMPOSE) up -d; \
		echo "${GREEN}Database reset complete!${RESET}"; \
	else \
		echo "${YELLOW}Operation cancelled.${RESET}"; \
	fi

.PHONY: db-backup
db-backup: ## Backup database to file
	@echo "${GREEN}Creating database backup...${RESET}"
	@mkdir -p backups
	$(DOCKER_COMPOSE) exec -T postgres pg_dump -U postgres -d hardware_store > backups/backup_$$(date +%Y%m%d_%H%M%S).sql
	@echo "${GREEN}Backup created in ./backups/${RESET}"

.PHONY: db-restore
db-restore: ## Restore database from file (usage: make db-restore file=backup.sql)
	@if [ -z "$(file)" ]; then \
		echo "${RED}Error: Please specify a backup file${RESET}"; \
		echo "Usage: make db-restore file=backups/backup_20240101_120000.sql"; \
		exit 1; \
	fi
	@if [ ! -f "$(file)" ]; then \
		echo "${RED}Error: File $(file) not found${RESET}"; \
		exit 1; \
	fi
	@echo "${YELLOW}Restoring database from $(file)...${RESET}"
	@cat $(file) | $(DOCKER_COMPOSE) exec -T postgres psql -U postgres -d hardware_store
	@echo "${GREEN}Database restored successfully!${RESET}"

.PHONY: db-schema
db-schema: ## Show database schema
	@echo "${GREEN}Database Schema:${RESET}"
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -d hardware_store -c "\dt"

.PHONY: db-tables
db-tables: ## List all tables
	@echo "${GREEN}Tables:${RESET}"
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -d hardware_store -c "\dt"

# ── Migration Commands ────────────────────────────────────────────────────────
.PHONY: migrate
migrate: ## Run database migrations
	@echo "${GREEN}Running migrations...${RESET}"
	$(DOCKER_COMPOSE) exec api ./main migrate
	@echo "${GREEN}Migrations complete!${RESET}"

.PHONY: migrate-status
migrate-status: ## Check migration status
	@echo "${GREEN}Migration status:${RESET}"
	$(DOCKER_COMPOSE) exec postgres psql -U postgres -d hardware_store -c "SELECT * FROM schema_migrations ORDER BY version;"

.PHONY: migrate-reset
migrate-reset: ## Reset migration state (WARNING: use with caution!)
	@echo "${RED}WARNING: This will reset the migration state!${RESET}"
	@read -p "Are you sure? (y/N) " -n 1 -r; \
	echo ""; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "${YELLOW}Resetting migration state...${RESET}"; \
		$(DOCKER_COMPOSE) exec postgres psql -U postgres -d hardware_store -c "TRUNCATE schema_migrations;"; \
		$(DOCKER_COMPOSE) exec postgres psql -U postgres -d hardware_store -c "INSERT INTO schema_migrations (version, dirty) VALUES (15, false);"; \
		echo "${GREEN}Migration state reset!${RESET}"; \
	else \
		echo "${YELLOW}Operation cancelled.${RESET}"; \
	fi

# ── Development Commands ──────────────────────────────────────────────────────
.PHONY: dev
dev: ## Start in development mode (with live logs)
	@echo "${GREEN}Starting in development mode...${RESET}"
	$(DOCKER_COMPOSE) up

.PHONY: test
test: ## Run tests
	@echo "${GREEN}Running tests...${RESET}"
	$(DOCKER_COMPOSE) exec api go test ./...

.PHONY: test-endpoints
test-endpoints: ## Run endpoint tests
	@echo "${GREEN}Testing endpoints...${RESET}"
	@if [ -f ./test_endpoints.sh ]; then \
		chmod +x ./test_endpoints.sh; \
		./test_endpoints.sh; \
	else \
		echo "${RED}test_endpoints.sh not found${RESET}"; \
	fi

.PHONY: shell
shell: ## Open a shell in the API container
	@echo "${GREEN}Opening shell in API container...${RESET}"
	$(DOCKER_COMPOSE) exec api /bin/sh

.PHONY: redis-cli
redis-cli: ## Open Redis CLI
	@echo "${GREEN}Opening Redis CLI...${RESET}"
	$(DOCKER_COMPOSE) exec redis redis-cli

.PHONY: lint
lint: ## Run linter
	@echo "${GREEN}Running linter...${RESET}"
	$(DOCKER_COMPOSE) exec api go vet ./...

# ── Cleanup Commands ──────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Clean up containers, networks, and volumes
	@echo "${RED}WARNING: This will remove all containers, networks, and volumes!${RESET}"
	@read -p "Are you sure? (y/N) " -n 1 -r; \
	echo ""; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "${YELLOW}Cleaning up...${RESET}"; \
		$(DOCKER_COMPOSE) down -v --rmi local; \
		$(DOCKER) system prune -f; \
		echo "${GREEN}Cleanup complete!${RESET}"; \
	else \
		echo "${YELLOW}Operation cancelled.${RESET}"; \
	fi

.PHONY: clean-logs
clean-logs: ## Clean up log files
	@echo "${YELLOW}Cleaning log files...${RESET}"
	$(DOCKER_COMPOSE) down
	$(DOCKER) system prune -f --filter "until=24h"
	@echo "${GREEN}Logs cleaned!${RESET}"

# ── Docker Management ─────────────────────────────────────────────────────────
.PHONY: prune
prune: ## Prune unused Docker resources
	@echo "${YELLOW}Pruning Docker resources...${RESET}"
	$(DOCKER) system prune -f
	$(DOCKER) volume prune -f
	@echo "${GREEN}Prune complete!${RESET}"

# ── Environment ───────────────────────────────────────────────────────────────
.PHONY: env
env: ## Show current environment variables
	@echo "${GREEN}Current environment:${RESET}"
	@$(DOCKER_COMPOSE) exec api env | grep -E "^(APP_|DB_|REDIS_|MPESA_|AIRTEL_|SMTP_|GREENAPI_|ENCRYPTION_|JWT_|PESAPAL_|CLOUDINARY_)" | sort

.PHONY: env-check
env-check: ## Check if all required environment variables are set
	@echo "${GREEN}Checking environment variables...${RESET}"
	@$(DOCKER_COMPOSE) exec api env | grep -E "^(JWT_SECRET|ENCRYPTION_KEY|DATABASE_URL|REDIS_URL)" | wc -l | xargs -I {} sh -c 'if [ {} -lt 4 ]; then echo "${RED}Missing required environment variables!${RESET}"; exit 1; else echo "${GREEN}All required variables set!${RESET}"; fi'

# ── Production Commands ───────────────────────────────────────────────────────
.PHONY: prod-up
prod-up: ## Start in production mode
	@echo "${GREEN}Starting in production mode...${RESET}"
	APP_ENV=production $(DOCKER_COMPOSE) up -d

.PHONY: prod-logs
prod-logs: ## Show production logs
	@echo "${GREEN}Showing production logs...${RESET}"
	APP_ENV=production $(DOCKER_COMPOSE) logs -f api

.PHONY: prod-build
prod-build: ## Build for production
	@echo "${GREEN}Building for production...${RESET}"
	APP_ENV=production $(DOCKER_COMPOSE) build --no-cache api

# ── Quick Commands ────────────────────────────────────────────────────────────
.PHONY: status
status: ps ## Alias for ps

.PHONY: stop
stop: down ## Alias for down

.PHONY: start
start: up ## Alias for up

.PHONY: api
api: api-status ## Alias for api-status

.PHONY: db
db: db-shell ## Alias for db-shell

.PHONY: seed-superadmin
seed-superadmin: ## Create the superadmin user
	@echo "${GREEN}Creating superadmin user...${RESET}"
	@docker-compose exec -T api go run scripts/seed_superadmin.go > /tmp/seed_superadmin.sql
	@docker-compose exec -T postgres psql -U postgres -d hardware_store < /tmp/seed_superadmin.sql
	@rm -f /tmp/seed_superadmin.sql
	@echo "${GREEN}Superadmin created!${RESET}"

# ── Help (default) ────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

# ── Catch-all for log targets ─────────────────────────────────────────────────
# This allows 'make logs api' to work
%:
	@: