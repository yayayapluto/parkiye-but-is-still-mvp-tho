# AI Agent Instructions — Parkiye-MVP

Parking management system. Go monorepo. Read this before making any changes.

---

## Quick Start

```bash
cp .env.example .env        # edit JWT_SECRET_KEY (min 32 chars) + QR_SECRET + S3_*
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
  modules/<n>/           # one folder per domain (see Module Layout below)
pkg/
  config/
    config.go               # Config structs + Load() + env helpers — S3Config, PLACE_NAME, QR_SECRET added
  qr/
    qr.go                   # Ticket PNG generator — 512x512, Go Mono font, HMAC filename obfuscation
  photo/
    photo.go                # Save(): multipart upload → S3-compatible storage (AWS Sig V4), returns publicURL + ocrPath
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
    auth.go                 # JWT auth middleware + GateAuth middleware + helpers
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
    transaction.go          # transaction-related enums + FlagTypePlateMismatch
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
`auth` · `zone` · `gate` · `vehicle` · `rfid` · `fee` · `transaction` · `payment` · `kiosk` · `override` · `ocr` · `audit`

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
12. GatePairingCode   ← gate pairing flow
```

Place new models **after** all their FK dependencies.
Never use `db.AutoMigrate(a, b, c)` in bulk — one at a time so FK errors surface clearly.

### Manual constraints

After AutoMigrate, `applyManualConstraints()` in `database/migrate.go` runs raw SQL for:

- Partial unique indexes (e.g. one active fee_config per zone+vehicle_type)
- CHECK constraints (date ranges, rate_type vs field, entry method vs rfid/qr)
- Performance indexes (transactions by status, ocr_jobs queued, audit by actor)

When changing schema: update **both** the model struct **and** the relevant SQL in `applyManualConstraints`.

### Date-only fields — gunakan `types.DateOnly`, bukan `time.Time`

`time.Time` hanya bisa unmarshal RFC3339 via `encoding/json`. Untuk field yang menerima tanggal saja (`YYYY-MM-DD`) dari
JSON request, gunakan `types.DateOnly`:

```go
// ✅ correct — menerima "2026-03-28" dari JSON
DateStart types.DateOnly `json:"date_start"`

// ❌ wrong — BodyParser akan return error untuk format YYYY-MM-DD
DateStart time.Time `json:"date_start"`
```

Untuk assign ke domain model (yang pakai `time.Time`), akses `.Time`:

```go
rate.DateStart = req.DateStart.Time
```

### Config (`pkg/config`)

```go
config.LoadEnv(".env") // call first in main(), before anything else
cfg, err := config.Load()
```

Required env (app panics without):

- `JWT_SECRET_KEY` — min 32 chars
- `QR_SECRET` — secret untuk HMAC obfuscation filename tiket parkir

Optional env (ada default):

- `PLACE_NAME` — nama tempat yang tampil di tiket (default: `Parkir`)

S3 storage env (wajib untuk upload foto):

- `S3_ENDPOINT` — e.g. `https://s3.nevaobjects.id`
- `S3_BUCKET` — nama bucket
- `S3_ACCESS_KEY` / `S3_SECRET_KEY`
- `S3_REGION` — default `us-east-1`
- `S3_PUBLIC_BASE_URL` — base URL publik untuk serve file, e.g. `https://parkieee.s3.nevaobjects.id`

### Photo Upload (`pkg/photo`)

```go
publicURL, ocrPath, err := photo.Save(fileHeader, "entry", cfg.S3)
// publicURL → "https://parkieee.s3.nevaobjects.id/entry_abc12345_1772000000.jpg"
// ocrPath   → sama dengan publicURL (OCR fetch via HTTP, bukan volume mount)
```

- Upload ke S3-compatible storage menggunakan AWS Signature V4 native (tanpa SDK)
- `EnsureBucketPolicy(cfg.S3)` dipanggil saat startup untuk set bucket public-read
- Foto diakses OCR via URL publik, bukan shared volume

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

### Gate Routes — ordering rule (Fiber)

Fiber evaluates routes sequentially. **Static segments must be registered before parametric ones** inside the same group:

```go
// ✅ correct — "/" and "/code/:code" registered before "/:id"
gateTx.Get("/", h.listTransactions)
gateTx.Get("/code/:code", h.getByCode)
gateTx.Get("/rfid/:uid", h.getOpenByRFID)
gateTx.Get("/:id", h.getTransaction)

// ❌ wrong — "/:id" would swallow "/" and "/code/:code"
gateTx.Get("/:id", h.getTransaction)
gateTx.Get("/", h.listTransactions)
```

Same rule applies to payment gate routes (`/gate/payments`).

### QRIS Payment — Polling vs Webhook

Midtrans webhook requires a public URL — does not work on `localhost`.
For local dev, the kiosk polls `GET /gate/payments/:id/poll` every 3 s.
`PollPaymentStatus` calls `midtrans.checkStatus(orderID)` directly and applies
`markQRISPaid` if Midtrans returns `settlement` or `capture`.
Webhook still works in production when a public tunnel (cloudflared) is active.

### Transaction — RFID exit fallback

`GetOpenByRFIDUID` first looks for `status = open`, then falls back to
`status = awaiting_payment` (card tapped again after exit already recorded).
Frontend also skips `recordExit` if the found transaction is already `awaiting_payment`.

### Simulation endpoint (dev only)

`PATCH /gate/payments/:id/simulate` body `{ "minutes_ago": N }` — backdates
`entry_at` of an open transaction. Gate-authenticated. Only works on `status = open`.

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

| Area                                 | Status                                                                              |
|--------------------------------------|-------------------------------------------------------------------------------------|
| `pkg/types`                          | ✅ done — FlagTypePlateMismatch ditambahkan                                          |
| `pkg/config`                         | ✅ done — S3Config added, StorageConfig masih ada (fallback)                         |
| `pkg/logger`                         | ✅ done                                                                              |
| `pkg/errors`                         | ✅ done                                                                              |
| `pkg/response`                       | ✅ done — paginated response, prev/next links always present                         |
| `pkg/validator`                      | ✅ done                                                                              |
| `pkg/metrics`                        | ✅ done                                                                              |
| `pkg/tracer`                         | ✅ done                                                                              |
| `pkg/middleware/auth.go`             | ✅ done — GateAuth + GateTokenValidator + GetGate* helpers                           |
| `pkg/helpers`                        | ⬜ empty — add utilities as needed                                                   |
| `pkg/types/date.go`                  | ✅ done — DateOnly type for YYYY-MM-DD JSON fields                                   |
| `pkg/photo/`                         | ✅ done — Save(): multipart → S3 via AWS Sig V4, EnsureBucketPolicy()               |
| `database/migrate.go`                | ✅ done — GatePairingCode ditambahkan                                                |
| `database/seed.go`                   | ✅ done — gofakeit, all 11 modules, 25–80 rows per entity                            |
| `internal/bootstrap/*`               | ✅ done: semua module sudah terwire (auth, zone, vehicle, rfid, fee, ocr, transaction, payment, gate) |
| `internal/modules/*/domain.go`       | ✅ done (all 11 modules)                                                              |
| `internal/modules/*/ports.go`        | ✅ done: auth, zone, vehicle, rfid, fee, transaction (incl. MarkPaid+MarkExited+MarkPaidAndExited+StampPlateMismatch), ocr (incl. TransactionStamperPort), payment, gate — override, audit: placeholder |
| `internal/modules/*/repository.go`   | ✅ done: auth, zone, vehicle, rfid, fee, transaction, ocr, payment, gate — override, audit: kosong |
| `internal/modules/*/service.go`      | ✅ done: auth, zone, vehicle, rfid, fee, transaction, ocr, payment, gate — override, audit: kosong |
| `internal/modules/*/dto.go`          | ✅ done: auth, zone, vehicle, rfid, fee, transaction, ocr, payment, gate — override, audit: placeholder |
| `internal/modules/*/handler.go`      | ✅ done: auth, zone, vehicle, rfid, fee, transaction, ocr, payment, gate — override, audit: kosong |
| `internal/modules/*/http_adapter.go` | ✅ done: auth, zone, vehicle, rfid, fee, transaction, ocr, payment, gate — override, audit: placeholder |
| `internal/modules/*/routes.go`       | ✅ done: auth, zone, vehicle, rfid, fee, transaction, ocr, payment, gate — override, audit: placeholder |
| `docs/Parkieee - Auth.*`             | ✅ done — biasa + full test suite                                                    |
| `docs/Parkieee - Zone.*`             | ✅ done — biasa + full test suite                                                    |
| `docs/Parkieee - Vehicle.*`          | ✅ done — biasa + full test suite                                                    |
| `docs/Parkieee - RFID.*`             | ✅ done — biasa + full test suite                                                    |
| `docs/Parkieee - Fee.*`              | ✅ done — full test suite (postman collection)                                       |
| `docs/Parkieee - Transaction.*`      | ✅ done — biasa + full test suite                                                    |
| `internal/modules/kiosk/*`           | ✅ done — `GET /api/v1/kiosk/tariff` (GateAuth), aggregate vehicle types + fee configs, filter by `zone.for_vehicle_type_id` |

**Next:** implement modules in dependency order:
`auth` ✅ → `zone` ✅ → `vehicle` ✅ → `rfid` ✅ → `fee` ✅ → `transaction` ✅ → `ocr` ✅ → `payment` ✅ → `gate` ✅ → `kiosk` ✅ → `override` → `audit`

### Gate API Summary (kiosk-facing endpoints)

```
# Transactions
GET    /gate/transactions                  list open/all (filter: status, page)
GET    /gate/transactions/:id              get by ID
GET    /gate/transactions/code/:code       get by QR code
GET    /gate/transactions/rfid/:uid        get open or awaiting_payment by RFID UID
POST   /gate/transactions/entry            record entry (multipart, optional photo)
POST   /gate/transactions/:id/exit         record exit (multipart, optional photo)
PATCH  /gate/transactions/:id/simulate     backdate entry_at (dev only)

# Payments
POST   /gate/payments/qris                 initiate QRIS (idempotent)
POST   /gate/payments/cash                 record cash intent
GET    /gate/payments/transaction/:txID    list payments for transaction
GET    /gate/payments/:id                  get payment by ID
GET    /gate/payments/:id/poll             check Midtrans status + update DB if paid

# Kiosk
GET    /kiosk/tariff                       get tariffs for gate's zone
```

---

## Photo & Storage

### `pkg/photo` — Save()

```go
publicURL, ocrPath, err := photo.Save(fileHeader, "entry", cfg.S3)
// publicURL → "https://parkieee.s3.nevaobjects.id/entry_abc12345_1772000000.jpg"
// ocrPath   → sama dengan publicURL
```

- Upload ke S3-compatible storage via raw AWS Signature V4 (bukan SDK)
- `EnsureBucketPolicy(cfg.S3)` dipanggil saat startup (`bootstrap/app.go`)
- OCR service fetch image via URL publik (bukan volume mount)
- Kalau S3 tidak dikonfigurasi, `signedPut` akan error — pastikan env S3_* terisi

### Transaction Handler — Photo Upload

Handler `recordEntry` dan `recordExit` di `transaction/handler.go` menerima foto via `multipart/form-data` (field: `photo`):

```go
if form, err := c.MultipartForm(); err == nil {
    if files := form.File["photo"]; len(files) > 0 {
        publicURL, volumePath, photoErr := photo.Save(files[0], "entry", h.s3cfg)
        // ...
    }
}
```

`handler` dan `httpAdapter` di transaction module menerima `config.S3Config` sebagai dependency.
`RegisterRoutes` juga menerima `s3cfg config.S3Config`.

### Gate Pairing — SSE

SSE state disimpan in-memory di `gate/repository.go` (map[code]chan + sync.RWMutex).
Bukan di DB — hanya survive selama proses Go berjalan.
Kalau server restart, screen perlu request pairing baru.

---

## Safety Rules for Agents

- **Never run `make db-reset`** unless explicitly asked — wipes all data
- **Never commit `.env`** — in `.gitignore`
- **Never use named Docker volume** untuk shared storage Go ↔ Python — photo sekarang di S3 (HTTP access)
- **Never batch AutoMigrate** — always one model at a time
- **Always add `column:` tag** on every new model field, especially acronyms
- **Always update `applyManualConstraints`** when adding composite uniques or CHECK constraints
- When unsure about module pattern, read the nearest completed module as reference
- **photo.Save() signature** sekarang `(file, prefix, s3cfg)` — bukan `(file, prefix, storageDir, ocrPrefix)`
- **Gate JWT** — `pairingRepo.Confirm` menerima plaintext JWT tapi menyimpan SHA-256 hash ke DB. JWT plaintext hanya dikirim via SSE channel. Jangan ubah behaviour ini.
- **Password complexity** — `CreateUser` dan `ChangePassword` wajib lolos `validatePasswordComplexity`: min 8 char, ada huruf, ada angka. Validasi dilakukan sebelum bcrypt.
- **Session cache** — `auth.service` punya `sessionCache` (sync.Map) TTL 60 detik dan `userRevokedAt` (sync.Map). Logout harus delete dari `sessionCache`. RevokeAll harus store ke `userRevokedAt`. Jangan bypass ini.
- **CORS** — production gunakan `APP_URL` env. Wildcard `*` hanya untuk non-production.
- **Gate JWT secret** — gunakan `cfg.GateJWTSecret()` (bukan `cfg.JWT.SecretKey` langsung) untuk gate token. Set `JWT_GATE_SECRET_KEY` di production.
- **OCR cross-module write** — OCR service tidak boleh menulis langsung ke tabel `transactions` via GORM. Gunakan `txStamper.StampPlateMismatch()` yang di-inject via `SetTransactionStamper`.
