.PHONY: help up down db-up db-down db-reset migrate migrate-seed seed tidy build run

## ── Help ─────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  PARKIEEE — available commands"
	@echo ""
	@echo "  Docker"
	@echo "    make up           Start all services (postgres + pgadmin)"
	@echo "    make down         Stop all services"
	@echo "    make db-up        Start postgres only"
	@echo "    make db-down      Stop postgres only"
	@echo "    make db-reset     Destroy postgres volume and recreate (wipes all data!)"
	@echo ""
	@echo "  Database"
	@echo "    make migrate      Run AutoMigrate (create/alter tables)"
	@echo "    make migrate-seed Run AutoMigrate + seeder"
	@echo "    make seed         Run seeder only (idempotent)"
	@echo ""
	@echo "  Go"
	@echo "    make tidy         go mod tidy"
	@echo "    make build        Build API binary"
	@echo "    make run          Run API (go run)"
	@echo ""

## ── Docker ───────────────────────────────────────────────────────────────────
up:
	docker compose up -d
	@echo "✓ Services running"
	@echo "  PostgreSQL → localhost:$$(grep DB_PORT .env | cut -d= -f2 | tr -d ' ' || echo 5432)"
	@echo "  pgAdmin    → http://localhost:$$(grep PGADMIN_PORT .env | cut -d= -f2 | tr -d ' ' || echo 5050)"

down:
	docker compose down

db-up:
	docker compose up -d postgres
	@echo "✓ PostgreSQL running on localhost:$$(grep DB_PORT .env | cut -d= -f2 | tr -d ' ' || echo 5432)"

db-down:
	docker compose stop postgres

db-reset:
	@echo "⚠️  This will DESTROY all data in postgres_data volume!"
	@read -p "Are you sure? [y/N]: " confirm && [ "$$confirm" = "y" ]
	docker compose down -v
	docker compose up -d postgres
	@echo "✓ PostgreSQL reset. Run 'make migrate-seed' to reinitialize."

## ── Database ─────────────────────────────────────────────────────────────────
migrate:
	go run ./cmd/migrate

migrate-seed:
	go run ./cmd/migrate -seed

seed:
	go run ./cmd/migrate -seed-only

## ── Go ───────────────────────────────────────────────────────────────────────
tidy:
	go mod tidy

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

## ── Setup (first time) ───────────────────────────────────────────────────────
setup:
	@if [ ! -f .env ]; then cp .env.example .env && echo "✓ .env created from .env.example — edit JWT_SECRET_KEY!"; fi
	@$(MAKE) tidy
	@$(MAKE) db-up
	@echo "⏳ Waiting for postgres to be ready..."
	@sleep 3
	@$(MAKE) migrate-seed
	@echo ""
	@echo "✅ Setup complete! Run 'make run' to start the API."