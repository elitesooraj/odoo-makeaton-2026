# Docker Compose is the build system. The host only needs Docker.
# Run `make help` for the list of targets.

SHELL := /bin/sh
COMPOSE := docker compose
PROD := -f docker-compose.yml -f docker-compose.prod.yml
# In the base file everything except `test` and `frontend` is default-profile,
# so plain `docker compose up` starts postgres, migrations, seed, api and
# mailpit. The prod override re-profiles mailpit and seed (dev/tools), which
# is why the explicit flags below remain for the tools/prod targets.
TOOLS_PROFILE := --profile tools
FRONTEND_PROFILE := --profile frontend

# Load local settings so COMPOSE_DATABASE_URL and the migration targets use the
# same credentials as the containers. Compose itself also reads .env.
-include .env
export

POSTGRES_USER ?= brightloop
POSTGRES_PASSWORD ?= brightloop
POSTGRES_DB ?= brightloop
COMPOSE_DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: help up up-prod bootstrap-prod down logs ps build build-prod frontend \
	db-up migrate-up migrate-down migrate-force migrate-create \
	seed seed-prod test test-integration vet fmt tidy config config-prod clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

up: ## Build and start the whole stack (migrations + seed run automatically)
	$(COMPOSE) up -d --build
	@echo ""
	@echo "API      http://localhost:$${API_PORT:-8080}"
	@echo "Mailpit  http://localhost:$${MAILPIT_UI_PORT:-8025}"

up-prod: ## Start the production stack (does not seed)
	$(COMPOSE) $(PROD) $(FRONTEND_PROFILE) up -d --build
	@echo "Run 'make seed-prod' once to create the bootstrap accounts."

bootstrap-prod: up-prod seed-prod ## Start production and seed once

down: ## Stop the stack (keeps the database volume)
	$(COMPOSE) $(TOOLS_PROFILE) $(FRONTEND_PROFILE) down --remove-orphans

logs: ## Follow API logs
	$(COMPOSE) logs -f api

ps: ## Show service status
	$(COMPOSE) ps

build: ## Build the local images
	$(COMPOSE) build

build-prod: ## Build the production images
	$(COMPOSE) $(PROD) build

frontend: ## Start the frontend behind the api
	$(COMPOSE) $(FRONTEND_PROFILE) up -d --build frontend

db-up: ## Start only postgres and mailpit
	$(COMPOSE) up -d postgres mailpit

migrate-up: ## Apply all migrations
	$(COMPOSE) run --rm --entrypoint migrate migrate \
		-path=/migrations -database "$(COMPOSE_DATABASE_URL)" up

migrate-down: ## Roll back one migration
	$(COMPOSE) run --rm --entrypoint migrate migrate \
		-path=/migrations -database "$(COMPOSE_DATABASE_URL)" down 1

migrate-force: ## Force a version: make migrate-force version=7
	@test -n "$(version)" || (echo "usage: make migrate-force version=<n>" && exit 1)
	$(COMPOSE) run --rm --entrypoint migrate migrate \
		-path=/migrations -database "$(COMPOSE_DATABASE_URL)" force $(version)

migrate-create: ## Create a migration pair: make migrate-create name=add_widgets
	@test -n "$(name)" || (echo "usage: make migrate-create name=<snake_case>" && exit 1)
	$(COMPOSE) run --rm --user "$$(id -u):$$(id -g)" --entrypoint migrate migrate \
		create -ext sql -dir /migrations -seq $(name)
	@echo "Rename the generated files into your reserved range (see docs/CONVENTIONS.md)."

seed: ## Re-run the idempotent seed job (runs automatically with `up`)
	$(COMPOSE) run --rm seed

seed-prod: ## Seed the production stack (idempotent)
	$(COMPOSE) $(PROD) $(TOOLS_PROFILE) run --rm seed

test: ## Run vet and unit tests in the build image (no database)
	$(COMPOSE) $(TOOLS_PROFILE) run --rm test

test-integration: ## Run vet, unit tests and DB integration tests against a disposable postgres
	$(COMPOSE) $(TOOLS_PROFILE) up -d --wait testdb
	$(COMPOSE) run --rm -e TEST_DATABASE_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@testdb:5432/brightloop?sslmode=disable" test go vet ./... && go test ./...
	$(COMPOSE) $(TOOLS_PROFILE) stop testdb

vet: ## Run go vet in the build image
	$(COMPOSE) $(TOOLS_PROFILE) run --rm test go vet ./...

fmt: ## Format Go code on the host files through the build image
	$(COMPOSE) $(TOOLS_PROFILE) run --rm \
		-v "$(CURDIR)/backend:/src" -w /src test gofmt -w .

tidy: ## Tidy modules on the host files through the build image
	$(COMPOSE) $(TOOLS_PROFILE) run --rm \
		-v "$(CURDIR)/backend:/src" -w /src test go mod tidy

config: ## Validate the local Compose configuration
	$(COMPOSE) config --quiet

config-prod: ## Validate the production Compose configuration
	$(COMPOSE) $(PROD) config --quiet

clean: ## Stop the stack and delete the database volume
	@echo "This deletes the postgres volume. Ctrl-C to abort."
	@sleep 3
	$(COMPOSE) $(TOOLS_PROFILE) $(FRONTEND_PROFILE) down -v --remove-orphans
