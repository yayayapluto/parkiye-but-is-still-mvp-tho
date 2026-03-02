# agents-logic.md — Full Business Logic Reference

Dokumen ini menjelaskan **semua logic bisnis yang sudah diimplementasi** di codebase ini.
Baca sebelum menambah atau mengubah apapun di service layer.

---

## Module: auth

### Login

```
1. Cari user by email
   └─ Tidak ada → return error generic "invalid email or password"
      (JANGAN expose "user not found" — selalu pesan generic)

2. Cek user.is_active dan user.deleted_at
   └─ Inactive/deleted → return "account is inactive"

3. Cek login_stats.is_locked
   └─ Locked → return "account is locked, contact administrator"
   (cek lockout SEBELUM verifikasi password — hemat bcrypt cost)

4. bcrypt.CompareHashAndPassword(user.password_hash, password)
   └─ Gagal → recordFailedAttempt() → return error generic

5. sessionRepo.DeleteExpired() — housekeeping ringan sebelum buat session baru

6. generateJWT(user) → token + expires_at
   JWT claims: sub (user_id), email, role, permissions[], exp, iat
   Permissions di-embed langsung di token dari user.role.permissions

7. Simpan session: {id, user_id, token_hash (SHA-256), ip, user_agent, expires_at}
   Token asli TIDAK PERNAH disimpan ke DB — hanya hash-nya

8. Append login_log (success=true)
9. resetLoginStats() — reset failed counter setelah login sukses
```

### Lockout

```
recordFailedAttempt():
  - Increment total_attempts, total_failed_attempts
  - Update last_attempt_at, last_failed_ip
  - Kalau total_failed_attempts >= 5 → is_locked = true, locked_at = now()
  - Upsert ke user_login_stats

resetLoginStats() setelah login sukses:
  - Reset total_failed_attempts = 0
  - is_locked = false
  - Update last_success_at
```

Lockout hanya bisa dibuka oleh admin (belum ada unlock endpoint — manual via DB atau buat endpoint admin).

### Token Validation (per request)

```
1. Parse JWT → verifikasi signature + expiry
2. Cari session by token_hash di DB
   └─ Tidak ada / sudah revoked → return 401
   (double check: token valid tapi session bisa sudah di-revoke via logout)
3. Return TokenClaims {user_id, email, role, permissions[]}
```

### Token Storage

Token di-hash SHA-256 sebelum disimpan. Format di DB: hex string dari SHA-256.

```go
h := sha256.Sum256([]byte(token))
tokenHash = hex.EncodeToString(h[:])
```

### Change Password

```
1. Verifikasi old_password dengan bcrypt
2. Hash new_password
3. Update user.password_hash
4. RevokeAllByUserID() — semua session di-revoke → user harus login ulang di semua device
```

### Deactivate User

```
- Tidak bisa deactivate diri sendiri (actorID == userID → error)
- SoftDelete: set deleted_at = now()
- RevokeAllByUserID() — kick semua session aktif
```

### Permission Check

Permission dicek dari JWT claims (in-memory, tidak hit DB).
Untuk operasi sensitif yang perlu real-time check, gunakan `auth.CheckPermission()` yang hit DB.

Middleware `RequirePermission("node")` cek dari `c.Locals("permissions")` — di-set saat token validation.

---

## Module: zone

### Capacity Tracking

Capacity tidak disimpan sebagai counter di zones table — dihitung dari `zone_capacity_logs` (append-only).

```
GetCurrentOccupancy(zoneID):
  SELECT occupied_count FROM zone_capacity_logs
  WHERE zone_id = ? ORDER BY recorded_at DESC LIMIT 1

available = zone.capacity - latest.occupied_count
```

```
RecordCapacityEvent(zoneID, transactionID, event):
  1. Ambil latest log untuk zone ini
  2. NextOccupancy(latest, capacity, event):
     - event = "entry" → occupied++, available--
     - event = "exit"  → occupied--, available++
     - Clamp: occupied tidak boleh < 0 atau > capacity
  3. Append baris baru (tidak update yang lama)
```

`RecordCapacityEvent` **dipanggil oleh transaction module** setelah entry/exit confirmed — bukan oleh zone module
sendiri.

### Zone CRUD

- Create: set `is_active = true`, simpan `created_by`
- Update: patch semantics — hanya field yang tidak nil yang diupdate
- Deactivate: set `is_active = false` (soft delete, data tidak hilang)
- Gate mengikuti zone — gate punya `zone_id` FK, tidak bisa berdiri sendiri

---

## Module: vehicle

### VehicleType

- `name` selalu di-lowercase dan di-trim saat create/update:
  ```go
  vt.Name = strings.ToLower(strings.TrimSpace(req.Name))
  ```

### Vehicle — Upsert Logic

```
UpsertVehicle(plate, vehicle_type_id, source):
  1. PlateNumber di-uppercase dan di-trim
  2. Cek VehicleType ada
  3. repo.Upsert():
     INSERT ... ON CONFLICT (plate_number) DO UPDATE SET vehicle_type_id, source, notes
  → Kalau plate sudah ada: update type/source saja, ID tidak berubah
  → Kalau plate baru: insert baru
```

Upsert dipakai oleh OCR module ketika plate terdeteksi dan perlu di-link ke transaction.

---

## Module: rfid

### RegisterOrGet — Idempotent

```
RegisterOrGet(card_uid):
  1. FindByUID(card_uid)
     → Ada → return existing (tidak buat baru, tidak error)
     → Tidak ada (ErrNotFound) → buat baru dengan is_active=true
  2. Kalau error bukan ErrNotFound → propagate error
```

Ini idempotent by design — scanner bisa kirim UID yang sama berkali-kali.

### LinkVehicle

```
LinkVehicle(card_id, vehicle_id):
  1. FindByID → cek card ada
  2. Cek is_active — card inactive tidak bisa di-link
  3. Set vehicle_id → Update
```

Card bisa di-unlink dengan set vehicle_id = null (via UpdateVehicle endpoint, bukan endpoint khusus).

### Deactivate

Set `is_active = false`. Card yang sudah inactive tidak bisa dipakai untuk entry baru (dicek di transaction module saat
scan).

---

## Module: fee

### Fee Config — Active Config Selection

```
FindActiveByZoneAndVehicle(zoneID, vehicleTypeID):
  WHERE zone_id = ? AND vehicle_type_id = ? AND is_active = true
    AND effective_from <= NOW()
    AND (effective_until IS NULL OR effective_until > NOW())
  ORDER BY effective_from DESC
  LIMIT 1
```

Yang paling baru (effective_from terbesar) menang. Partial unique index
`uidx_fee_configs_zone_vtype_active` mencegah dua config aktif untuk kombinasi yang sama.

### Tier Validation saat CreateFeeConfig

```
- Hanya boleh satu tier dengan is_last_tier = true
- Semua duration_minutes harus > 0
- Semua fee_amount harus >= 0
```

### CalculateFee

```
Input: zoneID, vehicleTypeID, entryTime, exitTime

1. Ambil active fee config untuk zone+vehicleType
2. total_minutes = exitTime - entryTime (menit, integer)
3. billable_minutes = total_minutes - grace_period_minutes
   └─ Kalau billable_minutes <= 0 → return base_fee (hanya bayar base)

4. raw_fee = base_fee + calculateTierFee(tiers, billable_minutes)

5. Ambil holiday rates aktif untuk tanggal entryTime
6. final_fee = applyHolidayRates(raw_fee, holiday_rates)

return final_fee
```

### calculateTierFee — Tier Walking Algorithm

```
Tiers diurutkan by tier_order (ascending).
remaining = billable_minutes

Untuk setiap tier:
  if remaining <= 0 → break

  if is_last_tier = true:
    blocks = ceil(remaining / duration_minutes)
           = (remaining + duration_minutes - 1) / duration_minutes
    total += blocks * fee_amount
    remaining = 0
    break

  if is_last_tier = false:
    consumed = min(remaining, duration_minutes)
    total += fee_amount   ← fee_amount adalah biaya FLAT per tier, bukan per menit
    remaining -= consumed

return total
```

Contoh: base=2000, tiers=[{60min, 3000, last=false}, {60min, 2000, last=true}], billable=150min

```
Tier 1: consumed=60, fee+=3000, remaining=90
Tier 2 (last): blocks=ceil(90/60)=2, fee+=2*2000=4000
Total tier fee = 7000
Raw fee = 2000 + 7000 = 9000
```

### applyHolidayRates

```
Kalau tidak ada holiday rate aktif → return raw_fee as-is

Kalau ada rate:
  Pisahkan menjadi dua grup: multiplier dan override

  Kalau ada override (meskipun ada multiplier juga):
    → Override SELALU menang atas multiplier
    → Kalau >1 override: rata-rata override_fee → return average
    → Ignore semua multiplier

  Kalau hanya multiplier:
    → Rata-rata semua multiplier
    → final_fee = raw_fee * average_multiplier (dibulatkan ke int)
```

### Holiday Rate Validation

```
rate_type = "multiplier":
  - multiplier wajib ada
  - multiplier > 0

rate_type = "override":
  - override_fee wajib ada
  - override_fee >= 0 (boleh nol untuk fee gratis)
```

### DateOnly Type

Field `date_start` dan `date_end` di holiday rate menggunakan `types.DateOnly`, bukan `time.Time`.
`time.Time` hanya bisa unmarshal RFC3339 dari JSON. `DateOnly` unmarshal format `YYYY-MM-DD`.

```go
// Di DTO: terima string "2026-03-28"
DateStart types.DateOnly `json:"date_start"`

// Di service: ekstrak ke time.Time untuk domain model
rate.DateStart = req.DateStart.Time
```

### Decimal dari JSON

`decimal.Decimal` tidak bisa unmarshal dari JSON number literal (`1.5`).
Gunakan `*float64` di DTO, konversi di service:

```go
// DTO
Multiplier *float64 `json:"multiplier"`

// Service
decimalPtrFromFloat(req.Multiplier) // *float64 → *decimal.Decimal
```

---

## Middleware & Auth Flow

### Request Lifecycle

```
Request masuk
  → Auth middleware:
      1. Ambil "Authorization: Bearer {token}" header
      2. ValidateToken():
         - Parse JWT
         - Cari session by token_hash di DB
      3. Set Locals: user_id, user_role, permissions[], user_email, claims
  → RequirePermission("node") middleware (kalau dipasang):
      - Cek permissions[] dari Locals
      - Tidak ada → 403
  → Handler
```

### Helpers

```go
middleware.GetUserID(c)       → uuid.UUID (uuid.Nil kalau tidak ada)
middleware.GetUserRole(c)     → string
middleware.GetPermissions(c)  → []string
middleware.GetClaims(c)       → *TokenClaims
```

### OptionalAuth

Untuk endpoint yang bisa diakses dengan atau tanpa login.
Tidak return error kalau token tidak ada — lanjut ke handler tanpa set Locals.

---

## Error Handling

Semua error menggunakan `pkg/errors` dengan kode terstandar:

```
ErrNotFound         → HTTP 404
ErrValidation       → HTTP 400
ErrInvalidCredentials → HTTP 401
ErrUnauthorized     → HTTP 401
ErrForbidden        → HTTP 403
ErrConflict         → HTTP 409
ErrInternal         → HTTP 500
ErrAccountLocked    → HTTP 423
```

```go
// Buat error baru
errors.New(errors.ErrNotFound, "fee config not found")

// Cek kode error
errors.IsCode(err, errors.ErrNotFound)

// Dari GORM result
errors.FromDB(result) // handle gorm.ErrRecordNotFound → ErrNotFound
```

Error dari GORM selalu di-wrap via `errors.FromDB()` di repository layer — service layer tidak perlu kenal GORM.

---

## Response Format

Semua response menggunakan `pkg/response`:

```go
// Success
response.Success(c, "message", data)        → 200
response.Created(c, "message", data)        → 201

// Error
response.BadRequest(c, "message", errs)     → 400
response.Unauthorized(c, "message")         → 401
response.Forbidden(c, "message")            → 403
response.NotFound(c, "message")             → 404

// Paginated
response.Paginated(c, "message", data, pagination)
```

Format paginated response selalu punya `prev` dan `next` links (null kalau tidak ada halaman sebelum/sesudah).

```json
{
  "success": true,
  "meta": {
    "code": "OK",
    "message": "ok"
  },
  "data": [
    ...
  ],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total": 42,
    "total_pages": 5,
    "prev": null,
    "next": "http://localhost:8080/api/v1/fee/configs?page=2&page_size=10"
  }
}
```

---

## Database Seed

Seed idempotent — semua insert pakai `ON CONFLICT DO NOTHING`.
Pengecekan `existing > 0` di awal setiap fungsi mencegah duplikasi saat re-seed.

### Fixed Data (Deterministic UUID)

UUID untuk data yang harus konsisten antar seed run:

```go
deterministicUUID(name) = uuid.NewSHA1(seedNamespace, []byte(name))
```

| Key                         | UUID                                          |
|-----------------------------|-----------------------------------------------|
| `"seed:admin"`              | Admin user tetap                              |
| `"role:operator"`           | Role operator                                 |
| `"role:admin"`              | Role admin                                    |
| `"role:owner"`              | Role owner                                    |
| `"role:engineer"`           | Role engineer                                 |
| `"perm:gate.override"`      | Permission gate.override                      |
| `"perm:fee.edit"`           | Permission fee.edit                           |
| `"perm:report.view"`        | Permission report.view                        |
| `"perm:user.manage"`        | Permission user.manage                        |
| `"perm:zone.manage"`        | Permission zone.manage                        |
| `"perm:rfid.manage"`        | Permission rfid.manage                        |
| `"perm:audit.read"`         | Permission audit.read                         |
| `"perm:config.edit"`        | Permission config.edit                        |
| `"vtype:motorcycle"`        | Vehicle type motor                            |
| `"vtype:car"`               | Vehicle type mobil                            |
| `"vtype:truck"`             | Vehicle type truk                             |
| `"zone:motor"`              | Zone Parkir Motor                             |
| `"zone:mobil"`              | Zone Parkir Mobil                             |
| `"zone:vip"`                | Zone Parkir VIP                               |
| `"ocr:config:default"`      | OCR config default (threshold 0.85)           |
| `"override:config:default"` | Override config default (max 10/day, 30/week) |

### Role Permission Matrix

| Role     | Permissions                                                                              |
|----------|------------------------------------------------------------------------------------------|
| operator | gate.override                                                                            |
| admin    | gate.override, fee.edit, report.view, user.manage, zone.manage, rfid.manage, config.edit |
| owner    | report.view, fee.edit, zone.manage, config.edit                                          |
| engineer | audit.read, config.edit                                                                  |

### Admin Default

```
Email    : admin@parkieee.local
Password : Admin@123!
Role     : admin
```

### Seed Order (dependency order)

```
roles → permissions → role_permissions → admin user → users
→ vehicle_types → vehicles
→ zones + gates
→ rfid_cards
→ fee_configs + tiers → holiday_rates
→ ocr_config → override_config
→ transactions → payments → operator_overrides → ocr_jobs + results
→ audit_logs
```

### TransactionCode Format (dari seed)

```
PKR-{YYYYMMDD}-{5digit_index}
Contoh: PKR-20260301-00001
```

Index hanya sequential dalam seed (`i+1`). Di production service, index dihitung dari jumlah transaksi hari ini + 1.

### Cash Payment — Round Up Logic

```go
roundUpToNearest(fee, 5000)
// fee = 7000 → tendered = 10000, change = 3000
// fee = 5000 → tendered = 5000,  change = 0
// fee = 12000 → tendered = 15000, change = 3000
```

---

## GORM Conventions

### column: Tag Wajib

Setiap field di domain model **harus punya explicit `column:` tag**, terutama:

- Akronim: `rfid_card_id`, `qr_code`, `url` — GORM kadang salah konversi
- Field dengan nama ambigu

```go
// ✅ benar
RFIDCardID *uuid.UUID `gorm:"type:uuid;column:rfid_card_id"`

// ❌ salah — GORM mungkin generate "r_f_i_d_card_id"
RFIDCardID *uuid.UUID `gorm:"type:uuid"`
```

### Soft Delete

Hanya `User` yang punya `deleted_at` (GORM soft delete).
Entity lain pakai `is_active = false` (deactivate pattern, bukan soft delete).

Jangan gunakan `gorm.Model` — semua field didefinisikan manual untuk kontrol penuh.

### AutoMigrate

Selalu migrate **satu model per baris** — jangan batch dalam satu panggilan:

```go
// ✅ benar
db.AutoMigrate(&auth.User{})
db.AutoMigrate(&auth.Role{})

// ❌ salah
db.AutoMigrate(&auth.User{}, &auth.Role{})
```

### Manual Constraints

Composite unique, partial unique, dan CHECK constraints didefinisikan di `applyManualConstraints()` di
`database/migrate.go` — tidak bisa dihandle GORM tag.

Contoh:

```sql
-- Partial unique: hanya satu fee config aktif per zone+vehicle_type
CREATE UNIQUE INDEX uidx_fee_configs_zone_vtype_active
    ON fee_configs (zone_id, vehicle_type_id) WHERE is_active = true;
```

---

## Types Package (`pkg/types`)

### DateOnly

Untuk field JSON yang menerima `YYYY-MM-DD` (bukan RFC3339):

```go
type DateOnly struct{ time.Time }

// Unmarshal: "2026-03-28" → time.Time midnight UTC
// Marshal: time.Time → "2026-03-28"
```

Gunakan di DTO. Di domain model tetap pakai `time.Time`.

### Enum Types

Semua tipe enum didefinisikan sebagai `type X string` di `pkg/types/`:

- `HolidayRateType`: `"multiplier"` | `"override"`
- `TransactionStatus`: `"open"` | `"awaiting_payment"` | `"paid"` | `"exited"` | `"overridden"` | `"cancelled"`
- `EntryMethod`: `"rfid"` | `"qr"`
- `ExitMethod`: `"rfid"` | `"qr"` | `"override"`
- `PaymentMethod`: `"cash"` | `"qris"`
- `PaymentStatus`: `"pending"` | `"completed"` | `"failed"` | `"expired"` | `"refunded"`
- `OCRJobStatus`: `"queued"` | `"processing"` | `"completed"` | `"failed"` | `"skipped"`
- `OverrideType`: `"lost_card_exit"` | `"no_qr_exit"` | `"fee_waive"` | `"fee_adjust"` | `"force_open_gate"` |
  `"manual_entry"`
- `GateType`: `"entry"` | `"exit"`
- `ZoneEventType`: `"entry"` | `"exit"`
- `TriggeredBy`: `"system"` | `"operator"` | `"cashier"` | `"webhook"`
- `FlagType`: `"overnight"` | `"multi_day"` | `"suspicious_duration"`
- `RefundStatus`: `"pending"` | `"approved"` | `"processed"` | `"rejected"`

---

---

## Module: ocr

### Dispatch Flow (fire-and-forget)

```
RecordEntry / RecordExit (transaction service)
  └─ EntryPhotoPath != "" / ExitPhotoPath != ""
       └─ go ocrSvc.DispatchOCRJob(ctx, transactionID, imagePath, zoneID, photoType)

DispatchOCRJob:
  1. Kalau OCR disabled → return (log + skip)
  2. Insert ocr_jobs (status=queued)
  3. processJob()
```

### processJob

```
1. Update status → processing
2. adapter.ping() — GET /health ke Python
   └─ Gagal → markJobSkipped (service unreachable)
3. Loop (0..maxRetries):
   result, err = adapter.callDetectPlate(imagePath)
   └─ Sukses → break
   └─ Gagal → log warn + retry
4. lastErr != nil → markJobFailed
5. handleSuccess()
```

### handleSuccess

```
1. Resolve vehicleTypeID dari zone.for_vehicle_type_id
   └─ Nil → skip vehicle upsert (log info, bukan error)
2. Set job.OutputImagePath + job.OutputImageURL (dari filename saja)
3. Auto-verify: confidence >= autoAcceptThreshold
4. vehicleSvc.UpsertVehicle(plate, vehicleTypeID, source=ocr)
5. Bandingkan PlateDetected vs vehicle.PlateNumber → IsMatch
6. resultRepo.Create(ocrResult)
7. updateTransactionVehicle() — link vehicle ke transaction (non-fatal)
8. Kalau photoType=exit → createReviewLog()
9. Update status → completed
```

### Python OCR HTTP Contract

```
POST /detect-plate
{ "image_path": "/mnt/storage/photos/entry_xxx.jpeg" }

→ 200: { "detected_plate": "B1701SGI", "confidence": 0.9831, "output_image_path": "/mnt/storage/photos/entry_xxx_output.jpeg" }
→ 404: { "detail": "Cannot find image" }
→ 422: { "detail": "No plate detected in the provided image" }
→ 500: { "detail": "Internal server error: ..." }
```

`image_path` adalah path **di dalam Docker container** (dari shared volume `/mnt/storage`).
`output_image_path` adalah path yang sama dengan `_output` suffix — Go ambil `filepath.Base()` dan jadikan URL publik.

### OCR Review Log (exit jobs only)

```
creatReviewLog():
  1. Cari entryResult by transactionID (resultRepo.FindEntryResultByTransactionID)
     └─ Tidak ada → log warn + return (non-fatal)
  2. autoMatch = normalizePlate(exitPlate) == normalizePlate(entryPlate)
  3. Insert ocr_review_logs {
       auto_match_result: true/false,
       ManualPlate/ReviewedBy/ReviewedAt: diisi operator nanti
     }
```

### normalizePlate

```go
// Hapus spasi, uppercase
normalizePlate("b 1701 sgi") == normalizePlate("B1701SGI") // true
```

### Config

```
OCR_ENABLED=true
OCR_API_URL=http://localhost:8001         # URL Python OCR service
OCR_TIMEOUT=15s
OCR_MAX_RETRIES=2
OCR_AUTO_ACCEPT_THRESHOLD=0.80           # confidence >= ini → auto-verified
STORAGE_DIR=./storage/photos             # local path untuk Go simpan foto
OCR_STORAGE_PREFIX=/mnt/storage/photos  # path container untuk Python OCR
```

---

## Logic yang Belum Diimplementasi (planned)

Lihat `agents-prepare.md` untuk detail lengkap. Ringkasan:

### Payment (planned)

- Cash: hitung kembalian, langsung complete
- QRIS: Midtrans API → webhook → validasi signature SHA-512
- Setiap webhook hit → log ke `midtrans_callbacks` dulu, baru proses
- Signature: `SHA512(order_id + status_code + gross_amount + server_key)`

### Override (planned)

- Cek daily/weekly limit dari OverrideConfig sebelum proses
- `fee_waive` → calculated_fee = 0 → MarkExited
- `force_open_gate` → mock (log saja, tidak ada hardware call)
