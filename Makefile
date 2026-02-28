.PHONY: help up down dev migrate migrate-seed seed tidy build run setup nih-orang refresh

include .env
export

MAKEFLAGS += --no-print-directory

help:
	@echo ""
	@echo "PARKIEEE - available commands"
	@echo "------------------------------------------------"
	@echo ""
	@echo "Docker"
	@echo "  make up           Start all services (postgres + observability)"
	@echo "  make down         Stop all services"
	@echo ""
	@echo "Development"
	@echo "  make dev          Start infrastructure + run API locally"
	@echo ""
	@echo "Database"
	@echo "  make migrate      Run AutoMigrate (create/alter tables)"
	@echo "  make migrate-seed Run AutoMigrate + seeder"
	@echo "  make seed         Run seeder only (idempotent)"
	@echo ""
	@echo "Go"
	@echo "  make tidy         go mod tidy"
	@echo "  make build        Build API binary"
	@echo "  make run          Run API locally (go run)"
	@echo "  make setup        First time setup (copy .env, migrate, seed)"
	@echo ""

up:
	docker compose up -d
	@echo "OK All services started"
	@echo "  PostgreSQL -> localhost:${DB_PORT}"
	@echo "  Prometheus -> http://localhost:${PROMETHEUS_PORT}"
	@echo "  Grafana    -> http://localhost:${GRAFANA_PORT}"
	@echo "  Loki       -> http://localhost:${LOKI_PORT}"

down:
	docker compose down -v
	@echo "OK All services stopped"

dev: migrate-seed up
	@clear
	@echo "------------------------------------------------"
	@echo "DEVELOPMENT MODE"
	@echo "------------------------------------------------"
	@echo ""
	@echo "Infrastructure (Docker):"
	@echo "  PostgreSQL : localhost:${DB_PORT}"
	@echo "  Prometheus : http://localhost:${PROMETHEUS_PORT}"
	@echo "  Grafana    : http://localhost:${GRAFANA_PORT}"
	@echo "  Loki       : http://localhost:${LOKI_PORT}"
	@echo ""
	@echo "API (Local):"
	@echo "  URL        : http://localhost:${SERVER_PORT}"
	@echo "  Metrics    : http://localhost:${SERVER_PORT}/metrics"
	@echo ""
	@echo "------------------------------------------------"
	@echo "Starting API locally..."
	@go run ./cmd/api

migrate:
	go run ./cmd/migrate

migrate-seed:
	go run ./cmd/migrate -seed

seed:
	go run ./cmd/migrate -seed-only

tidy:
	go mod tidy

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

refresh:
	@clear
	@$(MAKE) down
	@$(MAKE) up
	@clear
	@sleep 3
	@$(MAKE) run

setup:
	@if [ ! -f .env ]; then cp .env.example .env && echo "OK .env created from .env.example - edit JWT_SECRET_KEY!"; fi
	@$(MAKE) tidy
	@$(MAKE) up
	@echo "Waiting for postgres to be ready..."
	@sleep 3
	@$(MAKE) migrate-seed
	@echo ""
	@echo "OK Setup complete! Run 'make dev' to start development."

nih-orang:
	@echo "WAKAKAKAKKAKAKA"