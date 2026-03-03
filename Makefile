.PHONY: help up down wait-db dev migrate migrate-seed seed tidy kill build run refresh setup nih-orang

include .env
export
unexport STORAGE_DIR
unexport OCR_STORAGE_PREFIX

MAKEFLAGS += --no-print-directory

help:
	@echo ""
	@echo "PARKIEEE - available commands"
	@echo ""
	@echo "Docker"
	@echo "  make docker-up    Build + start all services including Go app (production-like)"
	@echo "  make docker-down  Stop all services (volumes preserved)"
	@echo "  make logs         Tail Go app logs"
	@echo "  make up           Start infrastructure only (postgres + observability, no app)"
	@echo "  make down         Stop all services + remove volumes"
	@echo ""
	@echo "Development"
	@echo "  make dev          Start infrastructure + migrate + seed + run API"
	@echo "  make refresh      Restart infrastructure + run API"
	@echo ""
	@echo "Database"
	@echo "  make migrate      Run AutoMigrate (create/alter tables)"
	@echo "  make migrate-seed Run AutoMigrate + truncate all tables + seeder"
	@echo "  make seed         Run truncate all tables + seeder"
	@echo ""
	@echo "Go"
	@echo "  make tidy         go mod tidy"
	@echo "  make kill         Kill running API process"
	@echo "  make build        Build API binary"
	@echo "  make run          Kill + build + run API"
	@echo "  make setup        First time setup (copy .env, migrate, seed)"
	@echo ""

up:
	docker compose up -d --scale app=0
	@echo "OK Infrastructure started (no app)"
	@echo "  Grafana    -> http://localhost:${GRAFANA_PORT}"
	@echo "  Prometheus -> http://localhost:${PROMETHEUS_PORT}"
	@echo "  Loki       -> http://localhost:${LOKI_PORT}"

docker-up:
	docker compose up -d --build
	@echo "OK All services started (including app)"
	@echo "  API        -> http://localhost:${SERVER_PORT}"
	@echo "  Grafana    -> http://localhost:${GRAFANA_PORT}"
	@echo "  Prometheus -> http://localhost:${PROMETHEUS_PORT}"
	@echo "  Loki       -> http://localhost:${LOKI_PORT}"

docker-down:
	docker compose down
	@echo "OK All services stopped (volumes preserved)"

logs:
	docker compose logs -f app

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
	go run ./cmd/migrate -seed -truncate

seed:
	go run ./cmd/migrate -seed-only -truncate

tidy:
	go mod tidy

kill:
	-taskkill //F //IM api.exe 2>NUL || true

build: tidy
	go build -o bin/api.exe ./cmd/api

run: kill
	./bin/api.exe

refresh: down up wait-db run

setup:
	@if [ ! -f .env ]; then cp .env.example .env && echo "OK .env created from .env.example - edit JWT_SECRET_KEY!"; fi
	@$(MAKE) tidy up wait-db migrate-seed
	@echo ""
	@echo "OK Setup complete! Run 'make dev' to start development."

nih-orang:
	@echo "WAKAKAKAKKAKAKA"
