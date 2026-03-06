# Parkiye MVP

Sistem manajemen parkir berbasis REST API yang dibangun dengan Go. Mendukung multi-zona, RFID, OCR plat nomor, manajemen
tarif, pembayaran, dan audit log — dilengkapi stack observability lengkap (metrics, logs, traces, profiling).

## Tech Stack

- **Runtime:** Go 1.25
- **Framework:** Fiber v2
- **Database:** PostgreSQL 16 + GORM
- **Auth:** JWT (access + refresh token)
- **Observability:** Prometheus · Grafana · Loki · Tempo · Pyroscope
- **Containerization:** Docker Compose

## Quick Start

```bash
cp .env.example .env       # wajib: edit JWT_SECRET_KEY (min 32 karakter)
make setup                 # tidy + start docker + migrate + seed
make dev                   # start infra + jalankan API lokal
```

Setelah `make dev`, API berjalan di `http://localhost:8080`.

  Default admin seed: `admin@parkieee.local` / `Admin@123!`

## Perintah Make

| Perintah            | Deskripsi                        |
|---------------------|----------------------------------|
| `make up`           | Jalankan semua Docker services   |
| `make down`         | Hentikan semua Docker services   |
| `make dev`          | Start infra + jalankan API lokal |
| `make migrate`      | Jalankan AutoMigrate             |
| `make migrate-seed` | AutoMigrate + seed data          |
| `make seed`         | Seed saja (idempotent)           |
| `make build`        | Build binary ke `bin/api`        |
| `make run`          | Jalankan API lokal (`go run`)    |
| `make tidy`         | `go mod tidy`                    |
| `make refresh`      | Restart services + jalankan API  |

## Struktur Project

```
cmd/
  api/main.go           # entrypoint API
  migrate/main.go       # CLI migrate + seed
database/
  database.go           # koneksi GORM + connection pool
  migrate.go            # AutoMigrate + manual constraints
  seed.go               # seed roles, permissions, admin
internal/
  bootstrap/            # inisialisasi app, server, DI container
  modules/              # domain modules (lihat bagian Modules)
pkg/
  config/               # env loading + config structs
  errors/               # AppError + error codes
  logger/               # structured logger (stdout text, file JSON)
  metrics/              # Prometheus metrics + HTTP middleware
  middleware/           # JWT auth middleware
  response/             # helper response + error handler + pagination
  tracer/               # OpenTelemetry tracing ke Tempo
  types/                # shared enums
  validator/            # validator + pesan error Bahasa Indonesia
scripts/
  prometheus.yml        # konfigurasi scrape Prometheus
  promtail.yml          # pengiriman log ke Loki
  tempo.yml             # konfigurasi Tempo
  grafana/provisioning/ # datasource + dashboard auto-provisioned
storage/logs/app.log    # log JSON (dibaca Promtail → Loki)
```

## Modules

Setiap modul mengikuti pola 8 file yang konsisten:

```
domain.go       # GORM model structs
ports.go        # interface RepoPort + ServicePort
repository.go   # implementasi RepoPort (GORM)
service.go      # business logic
dto.go          # request/response structs
handler.go      # Fiber handler functions
http_adapter.go # wiring handler ke router
routes.go       # registrasi routes
```

**Daftar modul:** `auth` · `zone` · `gate` · `vehicle` · `rfid` · `fee` · `transaction` · `payment` · `kiosk` · `override` ·
`ocr` · `audit`

## Observability

| Service            | URL                                        | Keterangan                |
|--------------------|--------------------------------------------|---------------------------|
| API                | `http://localhost:8080`                    | REST API                  |
| Fiber Monitor      | `http://localhost:8080/metrics`            | Dashboard bawaan Fiber    |
| Prometheus Metrics | `http://localhost:8080/metrics/prometheus` | Endpoint scrape           |
| Prometheus         | `http://localhost:9090`                    | Query metrics             |
| Grafana            | `http://localhost:3000`                    | Dashboard (admin / admin) |
| Loki               | `http://localhost:3100`                    | Log aggregation           |
| Tempo              | `http://localhost:3200`                    | Distributed tracing       |
| Pyroscope          | `http://localhost:4040`                    | Continuous profiling      |

Grafana sudah dikonfigurasi dengan datasource dan dashboard secara otomatis — tidak perlu setup manual.

## Environment Variables

Salin `.env.example` ke `.env` dan sesuaikan:

```env
# Wajib diubah
JWT_SECRET_KEY=ganti-dengan-secret-min-32-karakter

# Database (default cocok untuk Docker lokal)
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=parkieee

# Server
SERVER_PORT=8080

# Observability (opsional, bisa dimatikan di development)
ENABLE_TRACING=true
ENABLE_PROFILING=true
```

Lihat `.env.example` untuk daftar lengkap semua variabel.

## Database

- Semua PK menggunakan UUID (`gen_random_uuid()`)
- Semua timestamp menggunakan `TIMESTAMPTZ` dengan timezone UTC
- Soft delete hanya pada tabel `users` (via `deleted_at`), modul lain pakai flag `is_active`
- Schema dikelola via GORM AutoMigrate + manual constraints (partial unique index, CHECK constraints)

**Seed data bawaan:**

- Roles: `operator` · `admin` · `owner` · `engineer`
- Permissions: `gate.override` · `fee.edit` · `report.view` · `user.manage` · `zone.manage` · `rfid.manage` ·
  `audit.read` · `config.edit`
- Admin default: `admin@parkieee.local` / `Admin@123!`

## Development Notes

Beberapa konvensi penting saat menambah fitur:

**Selalu tambahkan tag `column:` eksplisit** pada setiap field model. GORM salah mengkonversi akronim kapital (`RFID` →
`rf_id`, `OCR` → `oc_r`).

```go
// ✅ benar
RFIDCardID *uuid.UUID `gorm:"column:rfid_card_id;type:uuid"`

// ❌ salah — GORM generate rf_id_card_id
RFIDCardID *uuid.UUID `gorm:"type:uuid"`
```

**Jangan batch AutoMigrate.** Selalu migrate satu model sekaligus agar error FK mudah dilacak.

**Urutan migrate harus mengikuti dependensi FK** — lihat `database/migrate.go` untuk urutan lengkapnya.

**Gunakan `pkg/response` untuk semua response handler**, jangan tulis `c.JSON(...)` langsung.

**Gunakan `pkg/errors.AppError`** untuk semua error yang dikembalikan dari service/repository, jangan return raw GORM
error ke handler.

## Kiosk Gate API

Endpoint khusus untuk kiosk (autentikasi gate token, bukan JWT user):

```
# Transactions
GET    /gate/transactions                  list transaksi (filter: status, page)
GET    /gate/transactions/:id              detail transaksi
GET    /gate/transactions/code/:code       cari by kode QR
GET    /gate/transactions/rfid/:uid        cari open/awaiting_payment by RFID
POST   /gate/transactions/entry            catat masuk (multipart, foto opsional)
POST   /gate/transactions/:id/exit         catat keluar (multipart, foto opsional)
PATCH  /gate/transactions/:id/simulate     mundurkan entry_at (dev only)

# Payments
POST   /gate/payments/qris                 buat QRIS (idempotent)
POST   /gate/payments/cash                 catat intent tunai
GET    /gate/payments/transaction/:txID    daftar payment per transaksi
GET    /gate/payments/:id                  detail payment
GET    /gate/payments/:id/poll             cek status Midtrans + update DB

# Kiosk
GET    /kiosk/tariff                       tarif zona gate (filter by vehicle type)
```

## Tunnel (Development)

```bash
# Backend
cloudflared tunnel run parkir-api

# Frontend kiosk (dari folder apps/kiosk)
pnpm tunnel   # = cloudflared tunnel --url http://localhost:5174
```