# AI Agent Instructions — Parkiye-MVP

Parking management system. Go monorepo. Read this before making any changes.

---

## Quick Start

```bash
cp .env.example .env        # edit JWT_SECRET_KEY (min 32 chars)
make setup                  # tidy + up + migrate + seed
make dev                    # start infra + run API locally
```

Individual commands:

```bash
make up                     # start all docker services
make down                   # stop all docker services
make migrate                # run AutoMigrate
make migrate-seed           # migrate + seed default data
make seed                   # seed only (idempotent)
make tidy                   # go mod tidy
make run                    # run API locally (go run)
make build                  # build binary to bin/api
make refresh                # restart services + run API
```

---

## Architecture

```
cmd/
  api/main.go               # API entry point
  migrate/main.go           # standalone migrate + seed CLI
database/
  database.go               # GORM + postgres connection + pool
  migrate.go                # AutoMigrate list + manual constraints
  seed.go                   # default roles, permissions, admin user
internal/
  bootstrap/
    app.go                  # App struct, NewApp(), Run()
    container.go            # Container struct, all module dependencies
    database.go             # initDatabase()
    logger.go               # newLogger() — maps config.Config → logger.Config
    observability.go        # initObservability() — Prometheus + OTel tracing
    server.go               # NewServer() — Fiber app, middleware, routes
  modules/<name>/           # one folder per domain (see Module Layout below)
pkg/
  config/
    config.go               # Config structs + Load() + env helpers
  errors/
    error.go                # AppError type + error codes
  helpers/                  # utilities — add as needed (currently empty)
  logger/
    config.go               # Config, FileConfig, SamplingConfig structs
    interface.go            # Logger interface + SlogLogger implementation
    handler.go              # teeHandler, samplingHandler (internal)
    middleware.go           # HTTP middleware, FromContext, GenerateTraceID
    new.go                  # New() constructor (stdout→text, file→JSON)
  metrics/
    metrics.go              # Prometheus counters/histograms/gauges
    middleware.go           # Fiber middleware — records HTTP metrics per request
  middleware/
    auth.go                 # JWT auth middleware
  response/
    response.go             # Response, Meta structs + Success/Created/BadRequest/etc
    errors.go               # ErrorHandler (AppError → fiber.Error → fallback)
    pagination.go           # Pagination structs + GeneratePagination + ParsePaginationRequest
  tracer/
    tracer.go               # OTel TracerProvider → Tempo via OTLP HTTP
  types/
    auth.go                 # auth-related enums
    gate.go                 # gate-related enums
    misc.go                 # shared enums
    payment.go              # payment-related enums
    transaction.go          # transaction-related enums
  validator/
    core.go                 # validator instance
    global.go               # global singleton
    helpers.go              # ValidateStruct helper
    options.go              # custom validation rules
    translator.go           # Indonesian/English error messages
scripts/
  prometheus.yml            # Prometheus scrape config
  promtail.yml              # Promtail → Loki log shipping config
  tempo.yml                 # Tempo single-binary config
  grafana/
    provisioning/
      datasources/
        datasources.yml     # Prometheus + Loki + Tempo datasources
      dashboards/
        dashboards.yml      # dashboard provider config
        parkieee-api.json   # HTTP metrics + logs dashboard
storage/
  logs/
    app.log                 # JSON log file (shipped to Loki via Promtail)
```

**Observability stack (Docker):**

- Prometheus → scrape `GET /metrics/prometheus` (port 9090)
- Grafana → dashboards auto-provisioned (port 3000)
- Loki + Promtail → reads `storage/logs/app.log` (JSON)
- Tempo → distributed tracing via OTLP HTTP port 4318 (UI port 3200)
- `GET /metrics` → Fiber built-in monitor dashboard

**Modules (domains):**
`auth` · `zone` · `gate` · `vehicle` · `rfid` · `fee` · `transaction` · `payment` · `override` · `ocr` · `audit`

---

## Module File Layout — always follow this pattern

Every module has the same 8 files. Mirror the nearest similar module when adding features.

```
domain.go       # GORM model structs — all fields must have explicit column: tag
ports.go        # interfaces: RepoPort + ServicePort
repository.go   # implements RepoPort using GORM
service.go      # implements ServicePort, business logic, calls repo
dto.go          # request/response structs with json: + validate: tags
handler.go      # Fiber handler functions, calls service, returns response
http_adapter.go # struct with handler dep, wires to Fiber router
routes.go       # registers routes on a fiber.Router
```

---

## Key Conventions

### Domain models — critical: explicit `column:` tag on every field

GORM splits consecutive uppercase acronyms incorrectly (`RFID` → `rf_id`, `OCR` → `oc_r`).
**Every field in every `domain.go` must have an explicit `column:` tag.**

```go
// ✅ correct
RFIDCardID *uuid.UUID `gorm:"column:rfid_card_id;type:uuid;index"`

// ❌ wrong — GORM generates rf_id_card_id
RFIDCardID *uuid.UUID `gorm:"type:uuid;index"`
```

### AutoMigrate order — strict FK dependency

`database/migrate.go` migrates models **one-by-one** (not batched) in this order:

```
1.  Role, Permission, User, RolePermission, UserSession, UserLoginLog, UserLoginStats
2.  Zone, Gate, GateDevice
3.  VehicleType, Vehicle
4.  RFIDCard
5.  FeeConfig, FeeTier, HolidayRate, OCRConfig, OverrideConfig
6.  Transaction, TransactionLog, UnclosedTransactionFlag
7.  Payment, MidtransCallback, Refund
8.  OperatorOverride
9.  OCRJob, OCRResult, OCRReviewLog
10. AuditLog, AuditLogExport
11. ZoneCapacityLog   ← last (depends on both zones and transactions)
```

Place new models **after** all their FK dependencies.
Never use `db.AutoMigrate(a, b, c)` in bulk — one at a time so FK errors surface clearly.

### Manual constraints

After AutoMigrate, `applyManualConstraints()` in `database/migrate.go` runs raw SQL for:

- Partial unique indexes (e.g. one active fee_config per zone+vehicle_type)
- CHECK constraints (date ranges, rate_type vs field, entry method vs rfid/qr)
- Performance indexes (transactions by status, ocr_jobs queued, audit by actor)

When changing schema: update **both** the model struct **and** the relevant SQL in `applyManualConstraints`.

### Config (`pkg/config`)

```go
config.LoadEnv(".env") // call first in main(), before anything else
cfg, err := config.Load()
```

Required env (app panics without):

- `JWT_SECRET_KEY` — min 32 chars

### Logger (`pkg/logger`)

stdout → text format (human-readable), file → JSON (Promtail-parseable). Both written simultaneously via teeHandler.

```go
// init in bootstrap/logger.go
log, err := logger.New(cfg.ToLoggerConfig())

// usage
log.Info(ctx, "msg", "key", value)
log.Error(ctx, "failed", "error", err)
log.With("module", "auth").Info(ctx, "login attempt")
log.WithGroup("request").Info(ctx, "incoming")
```

### Errors (`pkg/errors`)

Use `AppError` — not raw `errors.New`. Never return raw GORM errors to handlers.

```go
return nil, &errors.AppError{
Status:  fiber.StatusNotFound,
Code:    errors.ErrNotFound,
Message: "user not found",
}
```

### Response (`pkg/response`)

Never write `c.JSON(...)` directly in handlers. Always use `pkg/response`.

```go
// success
response.Success(c, "ok", data)
response.Created(c, "created", data)
response.Paginated(c, "ok", data, pagination)

// error
response.BadRequest(c, "invalid input", validationErrors)
response.NotFound(c, "user not found")
response.InternalError(c, "something went wrong")

// error handler (registered in fiber.Config)
fiber.Config{ErrorHandler: response.ErrorHandler}

// pagination
req := response.ParsePaginationRequest(c)
pagination := response.GeneratePagination(baseURL, route, req.Page, req.PageSize, total, queryParams)
```

### Validator (`pkg/validator`)

```go
v := validator.New()
if errs := v.ValidateStruct(dto); errs != nil {
return response.BadRequest(c, "validation failed", errs)
}
```

### Metrics (`pkg/metrics`)

Auto-recorded via middleware in `server.go`. No manual instrumentation needed for HTTP metrics.
For DB metrics: `metrics.DBQueryDuration.WithLabelValues("select").Observe(duration)`.

### Tracing (`pkg/tracer`)

Auto-recorded via `otelfiber.Middleware()` in `server.go`. For manual spans:

```go
ctx, span := tracer.Tracer("auth").Start(ctx, "login")
defer span.End()
```

### Payments

Every webhook hit → logged to `midtrans_callbacks` regardless of signature.
Never process a callback without checking `signature_valid = true` first.

---

## Database

- **Engine:** PostgreSQL 16 (Docker)
- **ORM:** GORM v1.25 + `gorm.io/datatypes` for jsonb
- **UUID:** all PKs use `gen_random_uuid()` (postgres native)
- **Timestamps:** always `TIMESTAMPTZ`, timezone UTC
- **Soft delete:** only `users` has `deleted_at` — others use `is_active` bool flag

Seed data (idempotent):

- 4 roles: `operator` · `admin` · `owner` · `engineer`
- 8 permissions: `gate.override` · `fee.edit` · `report.view` · `user.manage` · `zone.manage` · `rfid.manage` ·
  `audit.read` · `config.edit`
- Default admin: `admin@parkieee.local` / `Admin@123!`

---

## What's Done / What's Not

| Area                                 | Status                                                      |
|--------------------------------------|-------------------------------------------------------------|
| `pkg/types`                          | ✅ done                                                      |
| `pkg/config`                         | ✅ done                                                      |
| `pkg/logger`                         | ✅ done                                                      |
| `pkg/errors`                         | ✅ done                                                      |
| `pkg/response`                       | ✅ done — paginated response, prev/next links always present |
| `pkg/validator`                      | ✅ done                                                      |
| `pkg/metrics`                        | ✅ done                                                      |
| `pkg/tracer`                         | ✅ done                                                      |
| `pkg/middleware/auth.go`             | ✅ done                                                      |
| `pkg/helpers`                        | ⬜ empty — add utilities as needed                           |
| `database/migrate.go`                | ✅ done                                                      |
| `database/seed.go`                   | ✅ done — gofakeit, all 11 modules, 25–80 rows per entity    |
| `internal/bootstrap/*`               | ✅ done — container wires auth + zone + vehicle + rfid       |
| `cmd/api/main.go`                    | ✅ done                                                      |
| `internal/modules/*/domain.go`       | ✅ done (all 11 modules)                                     |
| `internal/modules/*/ports.go`        | ⬜ finished: auth, zone, vehicle, rfid                       |
| `internal/modules/*/repository.go`   | ⬜ finished: auth, zone, vehicle, rfid                       |
| `internal/modules/*/service.go`      | ⬜ finished: auth, zone, vehicle, rfid                       |
| `internal/modules/*/dto.go`          | ⬜ finished: auth, zone, vehicle, rfid                       |
| `internal/modules/*/handler.go`      | ⬜ finished: auth, zone, vehicle, rfid                       |
| `internal/modules/*/http_adapter.go` | ⬜ finished: auth, zone, vehicle, rfid                       |
| `internal/modules/*/routes.go`       | ⬜ finished: auth, zone, vehicle, rfid                       |
| `docs/Parkieee - Auth.*`             | ✅ done — biasa + full test suite                            |
| `docs/Parkieee - Zone.*`             | ✅ done — biasa + full test suite                            |
| `docs/Parkieee - Vehicle.*`          | ✅ done — biasa + full test suite                            |
| `docs/Parkieee - RFID.*`             | ✅ done — biasa + full test suite                            |

**Next:** implement modules in dependency order:
`auth` ✅ → `zone` ✅ → `vehicle` ✅ → `rfid` ✅ → `fee` → `transaction` → `payment` → `override` → `ocr` → `audit`

---

## Safety Rules for Agents

- **Never run `make db-reset`** unless explicitly asked — wipes all data
- **Never commit `.env`** — in `.gitignore`
- **Never batch AutoMigrate** — always one model at a time
- **Always add `column:` tag** on every new model field, especially acronyms
- **Always update `applyManualConstraints`** when adding composite uniques or CHECK constraints
- When unsure about module pattern, read the nearest completed module as reference