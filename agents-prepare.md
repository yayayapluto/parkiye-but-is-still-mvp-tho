# agents-prepare.md — Module Design: override, audit

Dokumen ini mendefinisikan rencana implementasi modul berikutnya sebelum dikerjakan.
Baca ini sebelum menyentuh kode apapun.

---

## Urutan Implementasi

```
transaction ✅ → ocr ✅ → payment ✅ → gate ✅ → override → audit
```

`override` depend ke `transaction` (via `TransactionID` FK).
`audit` berdiri sendiri — bisa dikerjakan paralel setelah `override`.

---

## Module: override

### Flow

1. Operator request override: `CreateOverride(transactionID, type, reason, approvedBy, ...)`
2. Cek `OverrideConfig` aktif — validasi daily/weekly limit operator
3. Kalau limit terlewat → return error + log (idealnya notify `escalation_notify_user_id` via log)
4. Insert `OperatorOverride`
5. Trigger aksi berdasarkan `override_type`:
    - `lost_card_exit` / `no_qr_exit` → `transaction.MarkOverridden` → buka gate (mock)
    - `fee_waive` → set `calculated_fee = 0` di transaction → `transaction.MarkExited`
    - `fee_adjust` → set `calculated_fee = adjusted_fee` → lanjut ke payment flow
    - `force_open_gate` → mock open gate signal
    - `manual_entry` → buat transaction baru secara manual

### ServicePort

```go
type ServicePort interface {
    CreateOverride(ctx context.Context, req CreateOverrideRequest, operatorID uuid.UUID) (*OperatorOverride, error)
    ListOverrides(ctx context.Context, filter OverrideFilter, page, pageSize int) ([]OperatorOverride, int64, error)
    GetOverride(ctx context.Context, id uuid.UUID) (*OperatorOverride, error)

    GetActiveOverrideConfig(ctx context.Context) (*OverrideConfig, error)
    CreateOverrideConfig(ctx context.Context, req CreateOverrideConfigRequest, createdBy uuid.UUID) (*OverrideConfig, error)
}
```

### Mock Gate

```go
func mockOpenGate(gateID uuid.UUID) {
    // log saja, tidak ada hardware call untuk MVP
}
```

### Routes

```
GET  /api/v1/overrides         → list (filter: operator_id, type, date)
GET  /api/v1/overrides/:id     → get
POST /api/v1/overrides         → create override (permission: gate.override)

GET  /api/v1/overrides/config  → get active config
POST /api/v1/overrides/config  → create config (permission: config.edit)
```

---

## Module: gate ✅

Sudah diimplementasi lengkap. Lihat `agents-logic.md` untuk detail.

Dua flow tersedia:
- **Token-first**: `POST /gate/authenticate` — screen input `gate_token` manual
- **QR Pairing**: screen generate QR → admin scan → confirm → screen dapat JWT via SSE

Endpoints:
```
POST /api/v1/gate/authenticate              — token-first (no auth)
POST /api/v1/gate/pairing/request           — screen request QR (no auth)
GET  /api/v1/gate/pairing/:code/listen      — SSE listener screen (no auth)
GET  /api/v1/gate/pairing/:code             — admin lihat info (auth required)
POST /api/v1/gate/pairing/:code/confirm     — admin confirm + assign gate (auth required)
```

Key implementation notes:
- `gate.ServicePort` implements `middleware.GateTokenValidator` — dipakai oleh `middleware.GateAuth()`
- SSE state in-memory di `pairingRepository` (map + mutex) — tidak survive server restart
- `GatePairingCode` domain ada di `gate/domain.go`, migrate di `database/migrate.go`
- `gate.NewService` menerima `(gateRepo zoneDomain.GateRepositoryPort, pairingRepo PairingRepositoryPort, cfg, log)`

---

## Module: transaction — photo upload via S3

Handler `recordEntry` dan `recordExit` menerima `config.S3Config` sebagai dependency.

```go
// http_adapter.go
type httpAdapter struct{ h *handler }

func newHTTPAdapter(svc ServicePort, v *validator.Validator, s3cfg config.S3Config) *httpAdapter {
    return &httpAdapter{h: newHandler(svc, v, s3cfg)}
}

// routes.go
func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator, s3cfg config.S3Config) {
    adapter := newHTTPAdapter(svc, v, s3cfg)
    // ...
}
```

`photo.Save(file, prefix, s3cfg)` — signature terbaru, upload ke S3 via AWS Sig V4.

---

## Ringkasan Dependencies di Container

```
TransactionService  ← FeeService, OCRService             (✅ terwire)
OCRService          ← VehicleService, ZoneRepo            (✅ terwire)
PaymentService      ← TransactionService                  (✅ terwire)
GateService         ← GateRepo, PairingRepo, Config       (✅ terwire)
OverrideService     ← TransactionService                  (planned — belum ada di container)
AuditService        − standalone                          (planned)
```

## Env Variables yang Sudah Ada

```
# S3 Storage (✅ done — gantikan STORAGE_DIR lama)
S3_ENDPOINT=https://s3.nevaobjects.id
S3_BUCKET=parkieee
S3_ACCESS_KEY=
S3_SECRET_KEY=
S3_REGION=us-east-1
S3_PUBLIC_BASE_URL=https://parkieee.s3.nevaobjects.id

# OCR (✅ done — fetch image via S3 URL, bukan volume)
OCR_ENABLED=true
OCR_API_URL=http://localhost:8001
OCR_TIMEOUT=15s
OCR_MAX_RETRIES=2
OCR_AUTO_ACCEPT_THRESHOLD=0.80

# Payment (✅ done)
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox

# Gate JWT (✅ done)
JWT_GATE_TOKEN_TTL=8760h   # 365 hari (default)
```

## Perubahan Breaking dari Versi Sebelumnya

| Area | Sebelum | Sekarang |
|------|---------|----------|
| `photo.Save()` signature | `(file, prefix, storageDir, ocrPrefix)` | `(file, prefix, s3cfg)` |
| Storage backend | Local filesystem | S3-compatible (NevaObjects) |
| OCR image access | Volume mount path | Public S3 URL via HTTP |
| `transaction.RegisterRoutes` | tanpa `s3cfg` | tambah `s3cfg config.S3Config` |
| `transaction.newHTTPAdapter` | tanpa `s3cfg` | tambah `s3cfg config.S3Config` |
