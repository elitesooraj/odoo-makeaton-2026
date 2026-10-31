DATABASE_URL ?= postgres://brightloop:brightloop@localhost:5432/brightloop?sslmode=disable
MIGRATE_IMAGE ?= migrate/migrate:v4.18.1
MIGRATIONS_DIR := $(CURDIR)/backend/migrations

.PHONY: help up down logs ps db-up migrate-up migrate-down migrate-force migrate-create seed api-dev test vet fmt tidy clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

up: ## Build and start postgres, mailpit, migrations and the API
	docker compose up -d --build

down: ## Stop the stack (keeps the database volume)
	docker compose down

logs: ## Follow API logs
	docker compose logs -f api

ps: ## Show service status
	docker compose ps

db-up: ## Start only postgres and mailpit
	docker compose up -d postgres mailpit

migrate-up: ## Apply all migrations (host database)
	docker run --rm --network host -v $(MIGRATIONS_DIR):/migrations $(MIGRATE_IMAGE) \
		-path=/migrations -database "$(DATABASE_URL)" up

migrate-down: ## Roll back one migration
	docker run --rm --network host -v $(MIGRATIONS_DIR):/migrations $(MIGRATE_IMAGE) \
		-path=/migrations -database "$(DATABASE_URL)" down 1

migrate-force: ## Force a version: make migrate-force version=7
	docker run --rm --network host -v $(MIGRATIONS_DIR):/migrations $(MIGRATE_IMAGE) \
		-path=/migrations -database "$(DATABASE_URL)" force $(version)

migrate-create: ## Create a migration pair: make migrate-create name=add_widgets
	docker run --rm -v $(MIGRATIONS_DIR):/migrations $(MIGRATE_IMAGE) \
		create -ext sql -dir /migrations -seq $(name)
	@echo "Rename the generated files into your reserved range (see docs/CONVENTIONS.md)."

seed: ## Create the admin and finance staff inside the running API container
	docker compose exec api /app/seed

api-dev: ## Run the API on the host (needs make db-up and a migrated database)
	@set -a; . ./.env; set +a; cd backend && go run ./cmd/api

test: ## Run unit tests
	cd backend && go test ./...

vet: ## Static checks
	cd backend && go vet ./...

fmt: ## Format Go code
	cd backend && gofmt -w .

tidy: ## Tidy modules
	cd backend && go mod tidy

clean: ## Remove build artifacts
	rm -rf backend/bin
