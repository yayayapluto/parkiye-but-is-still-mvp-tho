# Copilot / AI Agent Instructions — Parkiye-MVP

Purpose: help AI coding agents become productive quickly in this Go monorepo.

- Quick start
  - Run DB and services: `make db-up` or `make up` (uses `docker-compose.yml`). See [Makefile](Makefile#L1-L40).
  - Run migrations: `make migrate` (runs `cmd/migrate`). See [cmd/migrate/main.go](cmd/migrate/main.go#L1-L120).
  - Seed DB: `make seed` or `make migrate-seed`.
  - Build/run API: `make run` or `make build` + run binary. Entry point intended at [cmd/api/main.go](cmd/api/main.go) (file is currently empty).

- Big picture / architecture
  - Language: Go (module `parkieee`) — see [go.mod](go.mod#L1-L20).
  - Modular internal services: each domain lives under `internal/modules/<name>` with a consistent file layout: `domain.go`, `dto.go`, `handler.go`, `http_adapter.go`, `ports.go`, `repository.go`, `routes.go`, `service.go`. Use these conventions when adding features.
  - Database layer: GORM + Postgres. Connection and pool setup in [database/database.go](database/database.go#L1-L80). Migrations are centrally defined in [database/migrate.go](database/migrate.go#L1-L200) — respect the migration ordering there when adding models.
  - Configuration: environment-driven. Use `pkg/config` helpers. Load env with `pkg/config.LoadEnv()` before `pkg/config.Load()` (see [pkg/config/config.go](pkg/config/config.go#L1-L80)). `.env.example` documents required variables (notably `JWT_SECRET_KEY`).

- Project-specific patterns and conventions
  - Module layout: services expose `ports.go` interfaces and `service.go` implementations; `handler.go` contains business handlers; `http_adapter.go` wires handlers to HTTP transport; `routes.go` declares route registration. Follow this separation.
  - DB models & constraints: project relies on AutoMigrate plus manual SQL constraints in `applyManualConstraints` (in [database/migrate.go](database/migrate.go#L1-L200)). When changing schema, update both the model struct and any manual SQL constraints/indexes.
  - Migration ordering: the file enumerates a strict AutoMigrate order (auth, zone, vehicle, rfid, fee, transaction, payment, override, ocr, audit). Add new models into the correct position.
  - Logging / observability: optional observability services are present but commented in `docker-compose.yml` (Prometheus/Grafana/Loki). The project is MVP-focused — don't assume observability is enabled by default.

- Integration points & external deps
  - Postgres (primary DB) — see `docker-compose.yml` and `.env.example`.
  - Midtrans / payment callbacks referenced in `internal/modules/payment` (look for `MidtransCallback` models in [database/migrate.go](database/migrate.go#L1-L200)).

- Developer workflows for agents
  - Prefer small, focused changes. Use `make migrate` to verify DB migrations after adding models.
  - Run `make db-up` then `make migrate-seed` for local setup. Use `.env.example` to populate `.env` (Makefile `setup` target does this).
  - If modifying schema, update `applyManualConstraints` when you introduce composite uniques, partial indexes, or checks.

- Important files to inspect when working on a change
  - `Makefile` — developer commands and targets. ([Makefile](Makefile#L1-L120))
  - `cmd/migrate/main.go` — migration entry (migrate + seed). ([cmd/migrate/main.go](cmd/migrate/main.go#L1-L200))
  - `database/migrate.go` — migration model list and manual constraints. ([database/migrate.go](database/migrate.go#L1-L200))
  - `database/database.go` — GORM + Postgres setup. ([database/database.go](database/database.go#L1-L120))
  - `pkg/config/config.go` and `.env.example` — configuration contract. ([pkg/config/config.go](pkg/config/config.go#L1-L80), [.env.example](.env.example#L1-L30))
  - `internal/modules/*` — domain modules follow the same file layout; inspect nearby module to follow the pattern.

- Known repository state (to avoid wasted work)
  - `cmd/api/main.go` and `internal/bootstrap/*` are currently empty placeholders. Expect the API bootstrap to be incomplete; focus on `cmd/migrate` and database work when validating changes locally.

- Example tasks and where to start
  - Add a new DB model: add struct to `internal/modules/<module>/domain.go`, include it in `database/migrate.go` at the correct position, run `make migrate`.
  - Add HTTP route: add handler/service in the module, implement `http_adapter.go` and `routes.go` to register the endpoint; search other modules for a usage example.

- Safety & edit guidance for AI agents
  - Prefer non-destructive actions: avoid `db-reset` unless explicitly requested by the user. `db-reset` will wipe `postgres_data` volume (see `Makefile` target `db-reset`).
  - When patching files, keep edits minimal and follow existing file naming and package conventions.

If anything here is unclear or you want more details (examples from a specific module, or help wiring `cmd/api/main.go`), tell me which area to expand.
