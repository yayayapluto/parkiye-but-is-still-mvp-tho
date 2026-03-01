# agents-prepare.md — Module Design: transaction, payment, override, ocr

Dokumen ini mendefinisikan rencana implementasi 4 modul berikutnya sebelum dikerjakan.
Baca ini sebelum menyentuh kode apapun.

---

## Urutan Implementasi

```
transaction → payment → override → ocr
```

`payment`, `override`, dan `ocr` semua depend ke `transaction` (via `TransactionID` FK).
`ocr` di-queue oleh `transaction.CreateEntry`.

---

## Module: transaction

### Flow

**Entry:**

1. Gate scan RFID atau QR
2. `CreateEntry` → buat `Transaction` status `open`
3. Auto-queue `OCRJob` via `ocr.ServicePort.QueueJob` (dipanggil dari dalam transaction service)
4. Return transaction ke client

**Exit:**

1. Gate scan RFID atau QR di exit gate
2. `ProcessExit` → cari transaction `open` by RFID/QR
3. Hitung fee via `fee.ServicePort.CalculateFee(zoneID, vehicleTypeID, entryAt, exitAt)`
4. Update transaction: `exit_gate_id`, `exit_method`, `exit_at`, `calculated_fee`, `fee_config_id`, status →
   `awaiting_payment`
5. Insert `TransactionLog`
6. Return transaction ke client (client lanjut ke payment flow)

**Unclosed Flag (scheduler):**

- Goroutine + ticker jalan setiap 1 jam saat app start
- Query semua transaction status `open` dengan `entry_at` melewati threshold
- Threshold: > 8 jam → `overnight`, > 24 jam → `multi_day`, > 72 jam → `suspicious_duration`
- Insert `UnclosedTransactionFlag` kalau belum pernah di-flag untuk transaction itu

### ServicePort

```go
type ServicePort interface {
// Entry & Exit
CreateEntry(ctx, req CreateEntryRequest, operatorID uuid.UUID) (*Transaction, error)
ProcessExit(ctx, req ProcessExitRequest, operatorID uuid.UUID) (*Transaction, error)

// Read
GetByID(ctx, id uuid.UUID) (*Transaction, error)
GetByCode(ctx, code string) (*Transaction, error)
GetActiveByRFID(ctx, rfidCardID uuid.UUID) (*Transaction, error)
GetActiveByQR(ctx, qrCode string) (*Transaction, error)
ListTransactions(ctx, filter TransactionFilter, page, pageSize int) ([]Transaction, int64, error)

// Status transitions — dipanggil oleh payment/override module
MarkPaid(ctx, id uuid.UUID) error
MarkExited(ctx, id uuid.UUID) error
MarkOverridden(ctx, id uuid.UUID, note string) error
MarkCancelled(ctx, id uuid.UUID, reason string) error

// Receipt
MarkReceiptPrinted(ctx, id uuid.UUID) error

// Unclosed flag — dipanggil scheduler
FlagUnclosedTransactions(ctx) error
ListUnclosedFlags(ctx, resolved bool, page, pageSize int) ([]UnclosedTransactionFlag, int64, error)
ResolveFlag(ctx, flagID uuid.UUID, resolvedBy uuid.UUID, note string) error
}
```

### RepositoryPort

```go
type TransactionRepositoryPort interface {
Create(ctx, tx *Transaction) error
FindByID(ctx, id uuid.UUID) (*Transaction, error)
FindByCode(ctx, code string) (*Transaction, error)
FindActiveByRFID(ctx, rfidCardID uuid.UUID) (*Transaction, error)
FindActiveByQR(ctx, qrCode string) (*Transaction, error)
FindAll(ctx, filter TransactionFilter, page, pageSize int) ([]Transaction, int64, error)
Update(ctx, tx *Transaction) error

// Unclosed
FindOpenOlderThan(ctx, threshold time.Time) ([]Transaction, error)
}

type TransactionLogRepositoryPort interface {
Create(ctx, log *TransactionLog) error
FindByTransactionID(ctx, transactionID uuid.UUID) ([]TransactionLog, error)
}

type UnclosedFlagRepositoryPort interface {
Create(ctx, flag *UnclosedTransactionFlag) error
ExistsForTransaction(ctx, transactionID uuid.UUID) (bool, error)
FindAll(ctx, resolved bool, page, pageSize int) ([]UnclosedTransactionFlag, int64, error)
Resolve(ctx, id uuid.UUID, resolvedBy uuid.UUID, note string) error
}
```

### TransactionCode Generation

Format: `PKR-YYYYMMDD-{5digit_sequence}` → e.g. `PKR-20260301-00042`
Sequence: count transaksi hari ini + 1, zero-padded 5 digit.
Generate di service, bukan di DB.

### Domain model — column: tags

Semua field di `domain.go` transaction **sudah ada** tapi belum punya explicit `column:` tag.
Harus ditambahkan semua sebelum atau saat implementasi, khususnya:

- `RFIDCardID` → `column:rfid_card_id` (sudah ada di domain, tapi cek lagi)
- `FeeConfigID` → `column:fee_config_id`
- `HolidayRateID` → `column:holiday_rate_id`

### Routes

```
GET    /api/v1/transactions              → list (filter: status, zone_id, date)
GET    /api/v1/transactions/:id          → get by id
GET    /api/v1/transactions/code/:code   → get by code
POST   /api/v1/transactions/entry        → create entry
POST   /api/v1/transactions/exit         → process exit
PATCH  /api/v1/transactions/:id/receipt  → mark receipt printed
GET    /api/v1/transactions/:id/logs     → list transaction logs

GET    /api/v1/transactions/flags              → list unclosed flags
PATCH  /api/v1/transactions/flags/:id/resolve  → resolve flag
```

### Dependencies di Container

```go
TransactionRepo       transaction.TransactionRepositoryPort
TransactionLogRepo    transaction.TransactionLogRepositoryPort
UnclosedFlagRepo      transaction.UnclosedFlagRepositoryPort
TransactionService    transaction.ServicePort
```

Transaction service butuh inject: `fee.ServicePort`, `ocr.ServicePort` (untuk queue job).
Karena ocr depend ke transaction juga, hindari circular inject — ocr tidak di-inject ke transaction,
transaction hanya queue job via interface minimal atau channel.

**Solusi circular dependency:**
Transaction service menerima `OCRQueuer` interface yang tipis:

```go
type OCRQueuer interface {
QueueJob(ctx context.Context, transactionID uuid.UUID, imageURL string) error
}
```

OCR service implement interface ini. Di container, inject ocr service sebagai `OCRQueuer` ke transaction service.

---

## Module: payment

### Flow

**Cash:**

1. `PayCash(transactionID, cashTendered, handledByUserID)`
2. Ambil transaction, validasi status `awaiting_payment`
3. Hitung kembalian: `cash_change = cash_tendered - calculated_fee`
4. Validasi `cash_tendered >= calculated_fee`
5. Create `Payment` status `completed`, set `paid_at = now()`
6. Panggil `transaction.MarkPaid` lalu `transaction.MarkExited`

**QRIS:**

1. `InitiateQRIS(transactionID)`
2. Ambil transaction, validasi status `awaiting_payment`
3. Create `Payment` status `pending`
4. Call Midtrans API → dapat `order_id`, `qris_url`, `expires_at`
5. Update payment dengan Midtrans data
6. Return QRIS URL ke client

**Webhook:**

1. `POST /api/v1/payments/webhook/midtrans` — endpoint publik (no auth)
2. Log ke `midtrans_callbacks` **dulu** (selalu, apapun yang terjadi)
3. Validasi signature Midtrans
4. Kalau `signature_valid = false` → stop, jangan proses
5. Kalau valid → cari payment by `midtrans_order_id`
6. Update payment status sesuai Midtrans status
7. Kalau `settlement` atau `capture` → `transaction.MarkPaid` + `transaction.MarkExited`

**Refund:**

1. `RequestRefund(paymentID, reason, requestedBy)` → create Refund status `pending`
2. `ApproveRefund(refundID, approvedBy)` → butuh permission, call Midtrans refund API kalau QRIS, update status
   `processed`
3. `RejectRefund(refundID, approvedBy)` → update status `rejected`

### ServicePort

```go
type ServicePort interface {
PayCash(ctx, req PayCashRequest, handledBy uuid.UUID) (*Payment, error)
InitiateQRIS(ctx, transactionID uuid.UUID) (*Payment, error)
HandleMidtransWebhook(ctx, payload MidtransWebhookPayload) error

GetPayment(ctx, id uuid.UUID) (*Payment, error)
ListByTransaction(ctx, transactionID uuid.UUID) ([]Payment, error)

RequestRefund(ctx, req RequestRefundRequest, requestedBy uuid.UUID) (*Refund, error)
ApproveRefund(ctx, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error)
RejectRefund(ctx, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error)
ListRefunds(ctx, page, pageSize int) ([]Refund, int64, error)
}
```

### Midtrans Config (dari env)

```
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox   # "sandbox"|"production"
```

Tambahkan ke `pkg/config/config.go` dan struct `Config`.

### Routes

```
GET  /api/v1/payments/:id              → get payment
GET  /api/v1/payments/transaction/:id  → list by transaction
POST /api/v1/payments/cash             → pay cash
POST /api/v1/payments/qris             → initiate qris
POST /api/v1/payments/webhook/midtrans → webhook (NO AUTH)

POST  /api/v1/payments/refunds              → request refund
GET   /api/v1/payments/refunds              → list refunds
PATCH /api/v1/payments/refunds/:id/approve  → approve (permission: config.edit)
PATCH /api/v1/payments/refunds/:id/reject   → reject (permission: config.edit)
```

### Signature Validation Midtrans

```
SHA512(order_id + status_code + gross_amount + server_key)
```

Implementasi di service, bukan di handler.

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
CreateOverride(ctx, req CreateOverrideRequest, operatorID uuid.UUID) (*OperatorOverride, error)
ListOverrides(ctx, filter OverrideFilter, page, pageSize int) ([]OperatorOverride, int64, error)
GetOverride(ctx, id uuid.UUID) (*OperatorOverride, error)

GetActiveOverrideConfig(ctx) (*OverrideConfig, error)
CreateOverrideConfig(ctx, req CreateOverrideConfigRequest, createdBy uuid.UUID) (*OverrideConfig, error)
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

## Module: ocr

### Arsitektur

```
Transaction service
     │  QueueJob(transactionID, imageURL)
     ▼
OCR Service (Go)
     │  INSERT ocr_jobs status=queued
     │
     │  [goroutine worker, poll setiap 5 detik]
     │  HTTP POST → Python OCR service
     │             { job_id, image_path }
     │             ← { plate, confidence, raw }
     │
     ▼
OCRResult → auto-verify kalau confidence >= threshold
         → flag manual review kalau di bawah threshold
         → update transaction.vehicle_id kalau verified
```

### Shared Volume (Docker)

```yaml
volumes:
  parking_images:

services:
  api:
    volumes:
      - parking_images:/app/storage/images

  ocr:
    volumes:
      - parking_images:/app/storage/images
```

Go simpan image ke `/app/storage/images/{transaction_id}.jpg`.
Python baca dari path yang sama.
Go kirim `image_path` (string path) ke Python, bukan binary.

### Python OCR Contract

**Request:**

```json
POST /ocr
{
  "job_id": "uuid",
  "image_path": "/app/storage/images/{transaction_id}.jpg"
}
```

**Response:**

```json
{
  "plate": "B1234XYZ",
  "confidence": 0.94,
  "raw": {
    ...
  }
}
```

Python service belum ada — Go side mock response dulu dengan confidence 0.99 dan plate random, sampai Python service
siap.

### OCRQueuer Interface (anti-circular)

```go
// Di pkg atau di transaction package
type OCRQueuer interface {
QueueJob(ctx context.Context, transactionID uuid.UUID, imageURL string) error
}
```

OCR service implement interface ini. Di container:

```go
container.TransactionService = transaction.NewService(..., container.OCRService)
```

### Worker

```go
// Di bootstrap/app.go saat Run()
go container.OCRService.StartWorker(ctx)
```

Worker loop:

1. Query `ocr_jobs` status `queued`, limit 10
2. Update status → `processing`
3. HTTP POST ke Python service
4. Terima hasil → insert `OCRResult`
5. Compare confidence vs `OCRConfig.AutoAcceptThreshold`
6. Kalau >= threshold → `IsVerified = true`, update `transaction.vehicle_id`
7. Kalau < threshold → queue untuk manual review
8. Update `ocr_jobs` status → `completed` atau `failed`
9. Sleep 5 detik, repeat

### ServicePort

```go
type ServicePort interface {
// Implement OCRQueuer
QueueJob(ctx, transactionID uuid.UUID, imageURL string) error

// Worker
StartWorker(ctx context.Context)

// Manual review
ListPendingReviews(ctx, page, pageSize int) ([]OCRResult, int64, error)
SubmitManualReview(ctx, req SubmitReviewRequest, reviewedBy uuid.UUID) (*OCRReviewLog, error)

// Read
GetJob(ctx, id uuid.UUID) (*OCRJob, error)
GetResult(ctx, jobID uuid.UUID) (*OCRResult, error)
RetryJob(ctx, jobID uuid.UUID) error

// Config
GetActiveOCRConfig(ctx) (*OCRConfig, error)
CreateOCRConfig(ctx, req CreateOCRConfigRequest, createdBy uuid.UUID) (*OCRConfig, error)
}
```

### Routes

```
GET  /api/v1/ocr/jobs/:id             → get job
POST /api/v1/ocr/jobs/:id/retry       → retry failed job

GET  /api/v1/ocr/reviews              → list pending reviews
POST /api/v1/ocr/reviews/:result_id   → submit manual review

GET  /api/v1/ocr/config               → get active config
POST /api/v1/ocr/config               → create config (permission: config.edit)
```

---

## Ringkasan Dependencies di Container

```
TransactionService  ← FeeService, OCRService (as OCRQueuer)
PaymentService      ← TransactionService
OverrideService     ← TransactionService
OCRService          ← TransactionService (untuk update vehicle_id)
```

Inject order di `container.go`:

1. `initFeeModule`
2. `initOCRModule` (tanpa TransactionService dulu — hanya repo + worker stub)
3. `initTransactionModule` (inject FeeService + OCRService as OCRQueuer)
4. `initPaymentModule` (inject TransactionService)
5. `initOverrideModule` (inject TransactionService)
6. Setelah semua init, inject TransactionService ke OCRService via setter:
   `container.OCRService.SetTransactionService(container.TransactionService)`

## Env Variables Baru

Tambahkan ke `.env.example`:

```
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox
OCR_SERVICE_URL=http://ocr:8000
OCR_POLL_INTERVAL_SECONDS=5
OCR_MOCK=true   # kalau true, skip HTTP call ke Python, return mock result
```
