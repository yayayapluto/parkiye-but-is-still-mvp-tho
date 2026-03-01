.PHONY: help up down wait-db dev migrate migrate-seed seed tidy kill build run refresh setup nih-orang

include .env
export

MAKEFLAGS += --no-print-directory

help:
	@echo ""
	@echo "PARKIEEE - available commands"
	@echo ""
	@echo "Docker"
	@echo "  make up           Start all services (postgres + observability)"
	@echo "  make down         Stop all services"
	@echo ""
	@echo "Development"
	@echo "  make dev          Start infrastructure + migrate + seed + run API"
	@echo "  make refresh      Restart infrastructure + run API"
	@echo ""
	@echo "Database"
	@echo "  make migrate      Run AutoMigrate (create/alter tables)"
	@echo "  make migrate-seed Run AutoMigrate + seeder"
	@echo "  make seed         Run seeder only (idempotent)"
	@echo ""
	@echo "Go"
	@echo "  make tidy         go mod tidy"
	@echo "  make kill         Kill running API process"
	@echo "  make build        Build API binary"
	@echo "  make run          Kill + build + run API"
	@echo "  make setup        First time setup (copy .env, migrate, seed)"
	@echo ""

up:
	docker compose up -d
	@echo "OK All services started"
	@echo "  Grafana    -> http://localhost:${GRAFANA_PORT}"
	@echo "  Prometheus -> http://localhost:${PROMETHEUS_PORT}"
	@echo "  Loki       -> http://localhost:${LOKI_PORT}"

down:
	docker compose down -v
	@echo "OK All services stopped"

wait-db:
	@echo "Waiting for postgres to be ready..."
	@until docker compose exec -T postgres pg_isready -U ${DB_USER} -d ${DB_NAME} > /dev/null 2>&1; do \
		sleep 1; \
	done
	@echo "Postgres is ready"

dev: up wait-db migrate-seed
	@clear
	@echo "DEVELOPMENT MODE"
	@echo ""
	@echo "Infrastructure (Docker):"
	@echo "  PostgreSQL : localhost:${DB_PORT}"
	@echo "  Grafana    : http://localhost:${GRAFANA_PORT}"
	@echo "  Prometheus : http://localhost:${PROMETHEUS_PORT}"
	@echo "  Loki       : http://localhost:${LOKI_PORT}"
	@echo ""
	@echo "API (Local):"
	@echo "  URL        : http://localhost:${SERVER_PORT}"
	@echo "  Metrics    : http://localhost:${SERVER_PORT}/metrics"
	@echo ""
	@echo "Starting API locally..."
	@exec go run cmd/api/main.go

migrate:
	go run ./cmd/migrate

migrate-seed:
	go run ./cmd/migrate -seed

seed:
	go run ./cmd/migrate -seed-only

tidy:
	go mod tidy

kill:
	-taskkill //F //IM api.exe 2>NUL || true

build: tidy
	go build -o bin/api.exe ./cmd/api

run: kill build
	./bin/api.exe

refresh: down up wait-db run

setup:
	@if [ ! -f .env ]; then cp .env.example .env && echo "OK .env created from .env.example - edit JWT_SECRET_KEY!"; fi
	@$(MAKE) tidy up wait-db migrate-seed
	@echo ""
	@echo "OK Setup complete! Run 'make dev' to start development."

nih-orang:
	@echo "WAKAKAKAKKAKAKA"
