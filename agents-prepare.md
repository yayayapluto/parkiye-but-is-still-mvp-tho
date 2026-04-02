# agents-prepare.md — Rencana Implementasi Berikutnya

Dokumen ini mendefinisikan modul dan perubahan yang belum dikerjakan.
Baca ini sebelum menyentuh kode apapun. Update dokumen ini setiap kali sesuatu selesai atau keputusan berubah.

Terakhir diupdate: 2026-03-19

---

## Status Implementasi

```
transaction       ✅ selesai
ocr               ✅ selesai
payment           ✅ selesai
gate (pairing)    ✅ selesai
cashier-assign    ✅ selesai  ← schema, service, handler, routes, confirm pairing, SSE routing
gate-mode         ✅ selesai  ← GateMode type, schema migration, UpdateGateMode endpoint
mark-paid/exited  🔲 perlu dipisah  ← sekarang masih MarkPaidAndExited atomic
override          🔲 belum dimulai
audit             🔲 belum dimulai
object-detection  🔲 belum dimulai  ← Python service baru
```

---

## Urutan Prioritas (1 bulan ke UKK)

```
Minggu 1 ✅ SELESAI
  1. gate-mode schema + kolom di gates     ✅
  2. cashier-assign                         ✅
  3. confirm pairing update                 ✅
  4. /gate/payments/cashier/status          ✅
  5. fix TD-05 seed                         ✅

Minggu 2
  6. admin journey — /admin/users, /admin/zones, reassign kasir tanpa pairing ulang
  7. override module (minimal — lost_card_exit, fee_waive, manual_mark_exited)

Minggu 3
  8. pisah MarkPaid/MarkExited             ← prerequisite object detection
  9. object-detection Python service
  10. integrasi object detection → Go API

Minggu 4
  11. audit module (minimal)
  12. buffer + polish + dry run full flow end-to-end
```

---

## 1. Cashier Assignment per Exit Gate ✅ SELESAI

### Yang sudah diimplementasi

- `gate_cashier_assignments` table — UNIQUE on `gate_id`
- `GateCashierAssignmentRepositoryPort` — Upsert, FindByGateID, FindByUserID, DeleteByGateID
- Zone service — AssignCashier, UnassignCashier, GetCashierAssignment, GetAssignmentByUser
- Zone handler + routes — PUT/DELETE/GET `/zones/:id/gates/:gateId/cashier`
- `UpdateGateMode` — validasi ada assignment sebelum switch ke `with_cashier`
- Confirm pairing — kalau gate exit + mode `with_cashier`, insert assignment sekaligus
- Payment SSE routing — dari broadcast ke per-`userID` kasir, key di `cashierCh` sekarang userID
- `TouchCashierSeen` + `GetCashierStatus` — kasir dianggap online kalau lastSeenAt < 10 detik
- `/gate/payments/cashier/status` — kiosk poll sebelum tampilkan opsi tunai
- Permission baru: `cashier.assign`

### Keputusan arsitektur (final)

- 1 akun kasir = 1 exit gate, strict 1:1
- Assignment dilakukan oleh **admin** via `cashier.assign` permission
- Kasir login biasa (email/password) — tidak ada perubahan di auth flow
- SSE notif cash hanya dikirim ke kasir yang di-assign ke gate asal request
- Kalau kasir offline (`with_cashier` mode) → gate tidak beroperasi
- Reassign bisa tanpa pairing ulang

---

## 2. Gate Mode: manless vs with_cashier ✅ SELESAI

### Yang sudah diimplementasi

- `GateMode` type di `pkg/types/gate.go` — `manless` | `with_cashier`
- Kolom `mode` di tabel `gates` — DEFAULT `manless`
- `UpdateGateMode` endpoint: `PATCH /zones/:id/gates/:gateId/mode` (permission: `cashier.assign`)
- Mode masuk ke `GateInfo` JWT claims saat pairing
- `GateResponse` DTO sudah include `mode`
- GORM constraint `chk_gates_mode` untuk validasi nilai

### Behavior berdasarkan mode

| Mode | Payment | Mark Exited via | Fallback service down |
|---|---|---|---|
| `manless` | QRIS saja | object detection (wajib) | manual via panel override |
| `with_cashier` | QRIS + tunai | kasir konfirmasi atau object detection | gate tidak beroperasi |

### Endpoint cashier status

```
GET /gate/payments/cashier/status
Auth: gate JWT

Response:
  { "online": true,  "user_id": "uuid", "user_name": "Kasir A" }
  { "online": false, "user_id": "uuid", "user_name": "Kasir A" }
  { "online": false, "user_id": null }  ← belum ada assignment
```

Threshold online: `lastSeenAt` dalam 10 detik terakhir.

---

## 3. Module: Override 🔲 BELUM

### Konteks

Panel override di `apps/web` akan jadi "Swiss Army knife" operator/admin — satu tempat untuk semua kondisi darurat. Termasuk `manual_mark_exited` untuk transaksi stuck di `paid` saat object detection down (mode `manless`).

### Override types

| Override Type | Aksi | Siapa |
|---|---|---|
| `lost_card_exit` | `MarkOverridden` → buka gate | operator, admin |
| `no_qr_exit` | `MarkOverridden` → buka gate | operator, admin |
| `fee_waive` | set fee = 0 → `MarkExited` | operator, admin |
| `fee_adjust` | set fee = adjusted → lanjut payment | operator, admin |
| `force_open_gate` | log saja | operator, admin |
| `manual_entry` | buat transaksi manual | operator, admin |
| `manual_mark_exited` | manual trigger `MarkExited` untuk tx stuck di `paid` | operator, admin |

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

### Routes

```
GET  /api/v1/overrides              → list (filter: operator_id, type, date range)
GET  /api/v1/overrides/:id          → detail
POST /api/v1/overrides              → create (permission: override.perform)
GET  /api/v1/overrides/config       → get active config
POST /api/v1/overrides/config       → create config (permission: override.config)
```

---

## 4. Pisah MarkPaid dan MarkExited 🔲 BELUM

### Mengapa perlu dipisah

Sekarang `MarkPaidAndExited` atomic. Perlu dipisah karena:
- Palang buka setelah `MarkPaid`
- `MarkExited` dipicu berbeda tergantung mode gate:
  - `manless` → harus via object detection atau manual override
  - `with_cashier` → via kasir konfirmasi (`cashier/done`)

### Yang perlu diubah

`payment/service.go` — ganti semua call `txSvc.MarkPaidAndExited`:

```go
// Sebelum
txSvc.MarkPaidAndExited(ctx, txID, triggeredBy, handledByUserID)

// Setelah
if err := txSvc.MarkPaid(ctx, txID, triggeredBy, handledByUserID); err != nil {
    return err
}
// notify object detection service kalau mode = manless
// kalau mode = with_cashier, kasir yang trigger MarkExited via cashier/done
```

`MarkPaid` dan `MarkExited` sudah ada dan diimplementasi terpisah di `transaction/service.go`.

### Endpoint baru untuk object detection notify

```
POST /api/v1/internal/transactions/:id/mark-exited
Header: X-Internal-Token: <INTERNAL_API_TOKEN>
Body:   { "source": "object_detection", "confidence": 0.95 }
```

---

## 5. Object Detection Python Service 🔲 BELUM

### Arsitektur

Service baru terpisah dari Python OCR (port berbeda). FastAPI baru atau dijadikan satu codebase.

### Flow exit gate (mode manless)

1. Setelah `MarkPaid`, Go API kirim notif ke Python: `POST /api/watch-exit { transaction_id, gate_id }`
2. Python capture frame kamera gate
3. Selama kendaraan masih terdeteksi → tunggu
4. Kendaraan tidak terdeteksi → `POST /api/v1/internal/transactions/:id/mark-exited` ke Go API
5. Go API call `MarkExited`

### Behavior greeting

- Entry gate: kendaraan terdeteksi → trigger "Selamat Datang"
- Exit gate: kendaraan terdeteksi → trigger "Selamat Jalan", monitor sampai hilang → notify Go API (mode `manless` saja)

Tidak ada auto-close timeout — mode `manless` wajib manual intervention kalau detection down.

### Env variables baru

```
OBJECT_DETECTION_ENABLED=false
OBJECT_DETECTION_URL=http://localhost:8002
INTERNAL_API_TOKEN=
```

---

## 6. Module: Audit 🔲 BELUM

### Event yang perlu dicatat

- Login sukses / gagal
- Create / update / deactivate user
- Assign / reassign kasir ke gate
- Ganti mode gate
- Override dibuat
- Refund dibuat / disetujui / ditolak
- Fee config dibuat / diubah
- Gate di-pair / di-nonaktifkan

### ServicePort (minimal)

```go
type ServicePort interface {
    Log(ctx context.Context, event AuditEvent) error
    List(ctx context.Context, filter AuditFilter, page, pageSize int) ([]AuditLog, int64, error)
}
```

### Routes

```
GET /api/v1/audit   → list (filter: actor_id, event_type, date range)
                      Permission: audit.read
```

---

## Fix yang Perlu Dilakukan Sebelum Demo

### TD-05 — cashier.ability belum ada di seed ✅ FIXED

Sudah difix di `database/seed.go` dan `pkg/types/auth.go`. Verifikasi dengan jalankan seed ulang.

### TD-06 — gate name tampil UUID di tabel transaksi

Endpoint `GET /api/v1/gates` flat sudah ada (`router.Get("/gates", authMw, adapter.h.listAllGates)`). Frontend tinggal pakai untuk buat lookup map `gateId → gateName`.

---

## Ringkasan Dependencies di Container (updated)

```
TransactionService          ← FeeService, OCRService                   ✅ terwire
OCRService                  ← VehicleService, ZoneRepo                  ✅ terwire
PaymentService              ← TransactionService                        ✅ terwire
                              + GateCashierAssignmentRepo               ✅ terwire
GateService                 ← GateRepo, PairingRepo, Config             ✅ terwire
                              + GateCashierAssignmentRepo               ✅ terwire
ZoneService                 ← ZoneRepo, GateRepo, AssignmentRepo        ✅ terwire
OverrideService             ← TransactionService                        🔲 belum
AuditService                ← standalone                                🔲 belum
```

---

## Schema Changes Summary

Semua perubahan schema yang sudah masuk ke `database/migrate.go`:

```sql
-- gate mode
ALTER TABLE gates ADD COLUMN mode varchar(20) NOT NULL DEFAULT 'manless';

-- cashier assignment
CREATE TABLE gate_cashier_assignments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    gate_id     uuid NOT NULL REFERENCES gates(id),
    user_id     uuid NOT NULL REFERENCES users(id),
    assigned_by uuid NOT NULL REFERENCES users(id),
    assigned_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (gate_id)
);
```

GORM constraints yang sudah ditambahkan:
- `chk_gates_mode` — validasi nilai mode
- `chk_gate_cashier_assignments_exit_only` — assignment hanya untuk exit gate
- Index `idx_gate_cashier_assignments_user` dan `idx_transactions_paid_at`

---

## Env Variables Lengkap

```
# Database
DATABASE_URL=

# JWT
JWT_SECRET_KEY=
JWT_GATE_SECRET_KEY=
JWT_GATE_TOKEN_TTL=8760h

# S3
S3_ENDPOINT=https://s3.nevaobjects.id
S3_BUCKET=parkieee
S3_ACCESS_KEY=
S3_SECRET_KEY=
S3_REGION=us-east-1
S3_PUBLIC_BASE_URL=https://parkieee.s3.nevaobjects.id

# OCR
OCR_ENABLED=true
OCR_API_URL=http://localhost:8001
OCR_TIMEOUT=15s
OCR_MAX_RETRIES=2
OCR_AUTO_ACCEPT_THRESHOLD=0.80

# Object Detection
OBJECT_DETECTION_ENABLED=false
OBJECT_DETECTION_URL=http://localhost:8002

# Payment
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox

# App
APP_URL=
BASE_URL=
PLACE_NAME=

# Internal API (Python service → Go API)
INTERNAL_API_TOKEN=

# Cashier online detection threshold
CASHIER_ONLINE_THRESHOLD=10s
```
