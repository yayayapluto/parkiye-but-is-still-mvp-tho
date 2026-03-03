# agents-prepare.md — Module Design: transaction, payment, override, ocr

Dokumen ini mendefinisikan rencana implementasi 4 modul berikutnya sebelum dikerjakan.
Baca ini sebelum menyentuh kode apapun.

---

## Urutan Implementasi

```
transaction ✅ → ocr ✅ → payment 🔄 → override
```

`payment` dan `override` depend ke `transaction` (via `TransactionID` FK).

> **transaction dan ocr sudah selesai** — lihat `agents-logic.md` untuk detail implementasinya.

---

## Module: payment

### Status File

| File            | Status                                    |
|-----------------|-------------------------------------------|
| `domain.go`     | ✅ ada — perlu rename 1 field + tambah 1 field (lihat bawah) |
| `ports.go`      | ✅ ada — perlu update 1 signature          |
| `dto.go`        | ✅ ada — perlu update ikut rename domain   |
| `repository.go` | ⬜ kosong                                  |
| `service.go`    | ⬜ kosong                                  |
| `handler.go`    | ⬜ kosong                                  |
| `http_adapter.go` | ⬜ kosong                                |
| `routes.go`     | ⬜ kosong                                  |

---

### Perubahan domain.go yang Diperlukan

Rename + tambah field di struct `Payment`:

```go
// Sebelum
QRISUrl *string `gorm:"column:qris_url;type:text"`

// Sesudah — dua field terpisah
QRISString   *string `gorm:"column:qris_string;type:text"`   // raw QRIS string untuk render QR di client
QRISImageURL *string `gorm:"column:qris_image_url;type:text"` // hotlink image dari Midtrans actions[]
```

Alasannya: Midtrans response QRIS punya dua hal yang berbeda — `qr_string` (raw string, client render sendiri)
dan `actions[].url` (URL ke gambar QR yang dihosting Midtrans). Keduanya berguna untuk use case berbeda.

Update `database/migrate.go` dan `applyManualConstraints()` mengikuti perubahan kolom ini.

---

### Perubahan ports.go yang Diperlukan

Update signature `HandleMidtransWebhook` — handler yang parse body, rawBody dikirim ke service untuk disimpan ke
`MidtransCallback.RawPayload` apa adanya:

```go
// Sebelum
HandleMidtransWebhook(ctx context.Context, payload MidtransWebhookPayload) error

// Sesudah
HandleMidtransWebhook(ctx context.Context, rawBody []byte, payload MidtransWebhookPayload) error
```

---

### Perubahan yang Diperlukan di Luar Package payment

#### 1. `pkg/config/config.go`

Tambah struct dan field (bukan `getEnvRequired` — Midtrans opsional di dev):

```go
type MidtransConfig struct {
    ServerKey string // MIDTRANS_SERVER_KEY
    ClientKey string // MIDTRANS_CLIENT_KEY
    Env       string // MIDTRANS_ENV: "sandbox" | "production", default "sandbox"
}
```

Tambah `Midtrans MidtransConfig` di struct `Config`.

Load di `Load()`:
```go
Midtrans: MidtransConfig{
    ServerKey: getEnv("MIDTRANS_SERVER_KEY", ""),
    ClientKey: getEnv("MIDTRANS_CLIENT_KEY", ""),
    Env:       getEnv("MIDTRANS_ENV", "sandbox"),
},
```

#### 2. `internal/modules/transaction/ports.go`

Tambah dua method di `ServicePort`:

```go
// MarkPaid pindahkan status awaiting_payment → paid.
// handledByUserID nil untuk webhook (tidak ada user context).
MarkPaid(ctx context.Context, txID uuid.UUID, triggeredBy types.TriggeredBy, handledByUserID *uuid.UUID) error

// MarkExited pindahkan status paid → exited.
MarkExited(ctx context.Context, txID uuid.UUID, triggeredBy types.TriggeredBy) error
```

#### 3. `internal/modules/transaction/service.go`

Implementasi dua method baru. Pattern identik dengan `Cancel()` — DB transaction, update status, append log.

**`MarkPaid`:**
```
1. txRepo.FindByID
2. Validasi status == awaiting_payment → kalau bukan: return ErrValidation
3. db.Transaction:
   - fromStatus = string(existing.Status)
   - existing.Status = paid
   - txRepo.Update(ctx, tx, existing)
   - logRepo.Append(ctx, tx, &TransactionLog{
         FromStatus: &fromStatus,
         ToStatus: "paid",
         Event: types.EventPaymentReceived,
         TriggeredBy: triggeredBy,
         TriggeredByUserID: handledByUserID,
     })
4. return nil (tidak return *Transaction — caller tidak butuh)
```

**`MarkExited`:**
```
1. txRepo.FindByID
2. Validasi status == paid → kalau bukan: return ErrValidation
3. db.Transaction:
   - fromStatus = string(existing.Status)
   - existing.Status = exited
   - txRepo.Update(ctx, tx, existing)
   - logRepo.Append(ctx, tx, &TransactionLog{
         FromStatus: &fromStatus,
         ToStatus: "exited",
         Event: types.EventExitRecorded,
         TriggeredBy: triggeredBy,
     })
4. return nil
```

> **TIDAK** append capacity log di sini — capacity sudah dikurangi saat `RecordExit`
> (transisi open → awaiting_payment). MarkExited hanya status terminal, tidak ada perubahan occupancy.

---

### Midtrans HTTP Client (Internal, Tanpa SDK)

`github.com/midtrans/midtrans-go` tidak ada di `go.mod` dan tidak perlu ditambahkan.
Buat private struct di `internal/modules/payment/midtrans.go` (dalam package yang sama):

```go
type midtransClient struct {
    serverKey string
    baseURL   string // https://api.sandbox.midtrans.com atau https://api.midtrans.com
}

func newMidtransClient(cfg config.MidtransConfig) *midtransClient {
    base := "https://api.sandbox.midtrans.com"
    if cfg.Env == "production" {
        base = "https://api.midtrans.com"
    }
    return &midtransClient{serverKey: cfg.ServerKey, baseURL: base}
}
```

**`authHeader()`:**
```go
func (m *midtransClient) authHeader() string {
    encoded := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))
    return "Basic " + encoded
}
```

**`chargeQRIS(orderID string, grossAmount int) (*qrisChargeResponse, error)`:**

POST ke `/v2/charge`:
```json
{
  "payment_type": "qris",
  "transaction_details": {
    "order_id": "{orderID}",
    "gross_amount": {grossAmount}
  }
}
```

Response struct:
```go
type qrisChargeResponse struct {
    TransactionID     string `json:"transaction_id"`
    OrderID           string `json:"order_id"`
    QRString          string `json:"qr_string"`
    Actions           []struct {
        Name   string `json:"name"`
        Method string `json:"method"`
        URL    string `json:"url"`
    } `json:"actions"`
    TransactionStatus string `json:"transaction_status"`
    StatusCode        string `json:"status_code"`
    StatusMessage     string `json:"status_message"`
}
```

Return error kalau `StatusCode != "201"`.

`QRISImageURL` diambil dari `Actions` dimana `Name == "generate-qr-code"`, fallback ke `Actions[0].URL`
kalau tidak ada yang match.

**`refund(midtransTransactionID, refundKey string, amount int, reason string) error`:**

POST ke `/v2/{midtransTransactionID}/refund`:
```json
{
  "refund_key": "{refundKey}",
  "amount": {amount},
  "reason": "{reason}"
}
```

Return error kalau response status bukan 2xx.

---

### Service struct

```go
type service struct {
    repo      RepositoryPort
    txSvc     transaction.ServicePort // inject via ServicePort, bukan repo langsung
    midtrans  *midtransClient
    serverKey string // untuk verifikasi signature webhook
    log       logger.Logger
}

func NewService(
    repo RepositoryPort,
    txSvc transaction.ServicePort,
    cfg config.MidtransConfig,
    log logger.Logger,
) ServicePort
```

---

### Flow Detail

#### PayCash

```
1. tx := txSvc.GetTransaction(ctx, req.TransactionID)
   → validasi tx.Status == awaiting_payment
2. Validasi req.CashTendered >= *tx.CalculatedFee
   → error kalau kurang: "cash tendered is less than the required fee"
3. cashChange := req.CashTendered - *tx.CalculatedFee
4. now := time.Now()
5. repo.CreatePayment(&Payment{
       TransactionID:   req.TransactionID,
       Method:          types.PaymentMethodCash,
       Amount:          *tx.CalculatedFee,
       Status:          types.PaymentStatusCompleted,
       HandledByUserID: &handledBy,
       CashTendered:    &req.CashTendered,
       CashChange:      &cashChange,
       PaidAt:          &now,
   })
6. txSvc.MarkPaid(ctx, req.TransactionID, types.TriggeredByCashier, &handledBy)
7. txSvc.MarkExited(ctx, req.TransactionID, types.TriggeredByCashier)
8. return payment, nil
```

#### InitiateQRIS

```
1. tx := txSvc.GetTransaction(ctx, req.TransactionID)
   → validasi tx.Status == awaiting_payment
2. existing := repo.FindPaymentsByTransactionID(ctx, req.TransactionID)
   → scan slice: kalau ada yang Method=qris AND Status=pending → return existing (idempotent)
3. orderID := "PKR-" + uuid.New().String()
4. resp := midtrans.chargeQRIS(orderID, *tx.CalculatedFee)
5. expiresAt := time.Now().Add(15 * time.Minute)
   qrisString := resp.QRString
   imageURL := ambil dari resp.Actions[Name=="generate-qr-code"].URL atau Actions[0].URL
6. repo.CreatePayment(&Payment{
       TransactionID:         req.TransactionID,
       Method:                types.PaymentMethodQRIS,
       Amount:                *tx.CalculatedFee,
       Status:                types.PaymentStatusPending,
       MidtransOrderID:       &orderID,
       MidtransTransactionID: &resp.TransactionID,
       QRISString:            &qrisString,
       QRISImageURL:          &imageURL,
       QRISExpiresAt:         &expiresAt,
   })
7. return payment, nil
```

#### HandleMidtransWebhook

```
1. Build cb := &MidtransCallback{
       MidtransOrderID: payload.OrderID,
       RawPayload:      datatypes.JSON(rawBody),
       SignatureValid:  false,
       ReceivedAt:      time.Now(),
   }
2. repo.LogCallback(cb)  ← SELALU, sebelum apapun — audit trail

3. expected := SHA512(payload.OrderID + payload.StatusCode + payload.GrossAmount + serverKey)
   cb.SignatureValid = (expected == payload.SignatureKey)
4. repo.UpdateCallback(cb)

5. if !cb.SignatureValid → return nil
   (HTTP 200 ke Midtrans — jangan return error, Midtrans akan retry kalau non-2xx)

6. payment, err := repo.FindPaymentByMidtransOrderID(payload.OrderID)
   → err (NotFound): log warn + return nil
   → payment.Status == completed: return nil (idempotent — webhook bisa datang >1x)

7. switch payload.TransactionStatus:
   case "settlement":
       payment.Status = types.PaymentStatusCompleted
       now := time.Now(); payment.PaidAt = &now
   case "expire", "cancel", "deny", "failure":
       payment.Status = types.PaymentStatusFailed
   default:
       return nil  // "pending" dll — ignore

8. payment.MidtransStatus = &payload.TransactionStatus
9. repo.UpdatePayment(payment)

10. if payment.Status == completed:
    txSvc.MarkPaid(ctx, payment.TransactionID, types.TriggeredByWebhook, nil)
    txSvc.MarkExited(ctx, payment.TransactionID, types.TriggeredByWebhook)

11. now := time.Now()
    cb.Processed = true; cb.ProcessedAt = &now
    repo.UpdateCallback(cb)

12. return nil
```

**Signature verification:**
```go
func verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, incoming string) bool {
    raw := orderID + statusCode + grossAmount + serverKey
    h := sha512.New()
    h.Write([]byte(raw))
    return hex.EncodeToString(h.Sum(nil)) == incoming
}
```

Pakai `crypto/sha512` + `encoding/hex` — sudah ada di stdlib, tidak perlu dependency baru.

#### RequestRefund

```
1. payment := repo.FindPaymentByID(ctx, req.PaymentID)
   → validasi payment.Status == completed
2. Validasi req.RefundAmount <= payment.Amount
3. existing := repo.FindRefundByPaymentID(ctx, req.PaymentID)
   → kalau ada yang Status == pending atau Status == processed → return ErrConflict
     "there is already an active refund for this payment"
4. repo.CreateRefund(&Refund{
       PaymentID:    req.PaymentID,
       TransactionID: payment.TransactionID,
       RefundAmount: req.RefundAmount,
       Reason:       req.Reason,
       Status:       types.RefundStatusPending,
       RequestedBy:  requestedBy,
   })
5. return refund, nil
```

#### ApproveRefund

```
1. refund := repo.FindRefundByID(ctx, refundID)
   → validasi refund.Status == pending
2. payment := repo.FindPaymentByID(ctx, refund.PaymentID)
3. if payment.Method == qris:
   - refundKey := uuid.New().String()
   - midtrans.refund(*payment.MidtransTransactionID, refundKey, refund.RefundAmount, refund.Reason)
   - refund.MidtransRefundID = &refundKey
4. now := time.Now()
5. refund.Status = types.RefundStatusProcessed
   refund.ApprovedBy = &approvedBy
   refund.ProcessedAt = &now
6. repo.UpdateRefund(refund)
7. return refund, nil
```

> Refund QRIS pakai `midtrans_transaction_id` (bukan `order_id`) — ini yang benar per docs Midtrans sejak Jan 2024.

#### RejectRefund

```
1. refund := repo.FindRefundByID(ctx, refundID)
   → validasi refund.Status == pending
2. refund.Status = types.RefundStatusRejected
   refund.ApprovedBy = &rejectedBy
3. repo.UpdateRefund(refund)
4. return refund, nil
```

---

### Handler Notes

`webhookMidtrans` handler berbeda dari handler lain:
- Tidak pakai `middleware.GetUserID`
- Tidak pakai validator
- Baca raw body dulu: `rawBody := c.Body()`
- Unmarshal ke `MidtransWebhookPayload`
- Kirim keduanya ke service: `svc.HandleMidtransWebhook(c.Context(), rawBody, payload)`
- **Selalu return HTTP 200** — Midtrans retry kalau non-2xx

---

### Routes

```go
func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator) {
    adapter := newHTTPAdapter(svc, v)
    authMw := middleware.Auth(auth)
    managerMw := middleware.RequirePermission("config.edit")

    // Webhook HARUS di luar group authMw — Midtrans tidak kirim JWT
    router.Post("/payments/webhook/midtrans", adapter.h.webhookMidtrans)

    p := router.Group("/payments", authMw)
    // PENTING: route spesifik (/transaction/:txID) HARUS didaftarkan SEBELUM route parameter (/:id)
    // — Fiber akan match "transaction" sebagai nilai :id kalau urutannya terbalik
    p.Get("/transaction/:txID", adapter.h.listByTransaction)
    p.Get("/:id", adapter.h.getPayment)
    p.Post("/cash", adapter.h.payCash)
    p.Post("/qris", adapter.h.initiateQRIS)

    r := router.Group("/payments/refunds", authMw)
    r.Post("/", adapter.h.requestRefund)
    r.Get("/", adapter.h.listRefunds)
    r.Patch("/:id/approve", managerMw, adapter.h.approveRefund)
    r.Patch("/:id/reject", managerMw, adapter.h.rejectRefund)
}
```

---

### Wiring di bootstrap

**`container.go`** — tambah fields dan method:

```go
import "parkieee/internal/modules/payment"

// Di struct Container:
PaymentRepo    payment.RepositoryPort
PaymentService payment.ServicePort

// Method baru:
func (c *Container) initPaymentModule() error {
    c.PaymentRepo = payment.NewRepository(c.DB)
    c.PaymentService = payment.NewService(
        c.PaymentRepo,
        c.TransactionService,
        c.Config.Midtrans,
        c.Log,
    )
    return nil
}
```

Panggil `initPaymentModule()` di `NewContainer()` **setelah** `initTransactionModule()`.

**`server.go`** — tambah satu baris setelah `transaction.RegisterRoutes(...)`:

```go
payment.RegisterRoutes(api, container.PaymentService, container.AuthService, container.Validator)
```

---

### Env Variables

```
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox   # "sandbox" | "production"
```

---

### Checklist Eksekusi (urutan)

```
1. pkg/config/config.go              — tambah MidtransConfig
2. internal/modules/payment/domain.go — rename QRISUrl → QRISString + tambah QRISImageURL
3. internal/modules/payment/dto.go   — update PaymentResponse ikut rename
4. internal/modules/payment/ports.go — update HandleMidtransWebhook signature
5. internal/modules/transaction/ports.go — tambah MarkPaid + MarkExited
6. internal/modules/transaction/service.go — implementasi MarkPaid + MarkExited
7. internal/modules/payment/repository.go — implementasi RepositoryPort
8. internal/modules/payment/midtrans.go  — midtransClient (file terpisah dalam package)
9. internal/modules/payment/service.go  — implementasi ServicePort
10. internal/modules/payment/handler.go  — fiber handlers
11. internal/modules/payment/http_adapter.go — wrapper
12. internal/modules/payment/routes.go  — RegisterRoutes
13. internal/bootstrap/container.go    — initPaymentModule + wiring
14. internal/bootstrap/server.go       — payment.RegisterRoutes
```

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
    QueueJob(ctx context.Context, transactionID uuid.UUID, imageURL string) error

    // Worker
    StartWorker(ctx context.Context)

    // Manual review
    ListPendingReviews(ctx context.Context, page, pageSize int) ([]OCRResult, int64, error)
    SubmitManualReview(ctx context.Context, req SubmitReviewRequest, reviewedBy uuid.UUID) (*OCRReviewLog, error)

    // Read
    GetJob(ctx context.Context, id uuid.UUID) (*OCRJob, error)
    GetResult(ctx context.Context, jobID uuid.UUID) (*OCRResult, error)
    RetryJob(ctx context.Context, jobID uuid.UUID) error

    // Config
    GetActiveOCRConfig(ctx context.Context) (*OCRConfig, error)
    CreateOCRConfig(ctx context.Context, req CreateOCRConfigRequest, createdBy uuid.UUID) (*OCRConfig, error)
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
TransactionService  ← FeeService, OCRService   (✅ sudah terwire)
OCRService          ← VehicleService, ZoneRepo  (✅ sudah terwire)
PaymentService      ← TransactionService        (🔄 sedang diimplementasi)
OverrideService     ← TransactionService        (planned)
```

## Env Variables yang Sudah Ada

```
# OCR (✅ done)
OCR_ENABLED=true
OCR_API_URL=http://localhost:8001
OCR_TIMEOUT=15s
OCR_MAX_RETRIES=2
OCR_AUTO_ACCEPT_THRESHOLD=0.80
STORAGE_DIR=./storage/photos
OCR_STORAGE_PREFIX=/mnt/storage/photos

# Payment (🔄 sedang diimplementasi)
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox
```
