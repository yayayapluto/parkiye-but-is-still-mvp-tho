---
description: 
---

> Baca sebelum sentuh file. Digunakan untuk: audit kode baru, PR critique, modul review, debt identifikasi.

## ✅ Cara Pakai
1. Baca file via Filesystem MCP  
2. Jalankan checklist sesuai scope  
3. Laporkan: `[SEVERITY] path:baris — masalah — solusi`  
4. Jangan modifikasi tanpa konfirmasi  

**Severity:**  
- `CRITICAL`: bug, data corruption, security, crash  
- `WARNING`: pelanggaran arsitektur/pola, risiko prod  
- `SUGGESTION`: clean/idiomatic/maintainable  
- `NITPICK`: style/naming/komentar — boleh diabaikan  

---

## 1. Arsitektur — Module Layout (`internal/modules/<name>/`)
Wajib 8 file:
```
domain.go       — GORM models + TableName()
ports.go        — RepoPort + ServicePort interfaces
repository.go   — GORM-only impl
service.go      — business logic, call repo
dto.go          — request/response (json+validate)
handler.go      — Fiber handlers, call service
http_adapter.go — wire handler ke router
routes.go       — RegisterRoutes(router, svc, v)
```

**Flag:**
| Violation | Severity |
|---|---|
| Handler akses GORM/DB langsung | CRITICAL |
| Service return `c.JSON(...)` / depend Fiber | CRITICAL |
| Repository punya logic bisnis | WARNING |
| Handler punya kalkulasi bisnis | WARNING |
| File tambahan tanpa alasan jelas | SUGGESTION |
| Module baru tidak mirror pola exist | WARNING |

---

## 2. Domain Models (`domain.go`)
### ⚠️ `column:` tag WAJIB  
GORM salah konversi akronim: `RFID` → `rf_id`.  
✅ `RFIDCardID *uuid.UUID `gorm:"column:rfid_card_id;type:uuid;index"`  
❌ `RFIDCardID *uuid.UUID `gorm:"type:uuid;index"`  

**Checklist:**
- [ ] Semua field punya `column:`  
- [ ] FK: `type:uuid` + index  
- [ ] `TableName()` didefinisikan  
- [ ] Soft delete hanya di `users` → lainnya `is_active bool`  
- [ ] Timestamp: `TIMESTAMPTZ`, bukan `DATE`/`VARCHAR`  
- [ ] Date-only: `types.DateOnly`, bukan `time.Time`  
→ Akses `.Time` saat assign ke domain

---

## 3. Errors (`pkg/errors`)
Jangan return raw GORM / `errors.New()` / `fmt.Errorf`.

✅ `return nil, errors.New(errors.ErrNotFound, "user not found")`  
✅ `return nil, errors.Wrap(err, errors.ErrDatabaseError, "...")`  
❌ `return nil, err` / `fmt.Errorf("not found")` / `gorm.ErrRecordNotFound`

**Kode error utama:**
| Code | HTTP | Kapan |
|---|---|---|
| `ErrInvalidRequest` | 400 | body/query salah |
| `ErrValidation` | 400 | validator gagal |
| `ErrUnauthorized` | 401 | token tidak ada/invalid |
| `ErrInvalidCredentials` | 401 | password salah |
| `ErrAccountLocked` | 401 | akun terkunci |
| `ErrForbidden` | 403 | role tidak cukup |
| `ErrNotFound` | 404 | resource tidak ditemukan |
| `ErrConflict`/`ErrDuplicate` | 409 | duplikat |
| `ErrResourceInUse` | 409 | masih dipakai |
| `ErrInternal`/`ErrDatabaseError` | 500 | unexpected / DB gagal |
| `ErrExternalService` | 502 | Midtrans/S3/OCR gagal |

**Checklist:**
- [ ] Tidak ada `fmt.Errorf` ke handler tanpa wrap  
- [ ] Tidak ada `gorm.ErrRecordNotFound` bocor  
- [ ] `Wrap` dipakai jika ingin preserve underlying err  
- [ ] Status 500 tidak expose message teknis ke client  

---

## 4. Response (`pkg/response`)
Jangan `c.JSON(...)` langsung di handler.

✅ `response.Success(c, "ok", data)`  
✅ `response.Paginated(c, "ok", items, pagination)`  
❌ `c.JSON(fiber.Map{...})` / `c.Status(200).JSON(...)`  

**Checklist:**
- [ ] Semua handler pakai `response.*` atau `return err`  
- [ ] Pagination: `ParsePaginationRequest` + `GeneratePagination`  
- [ ] List endpoint selalu `Paginated`, bukan `Success` dengan slice  
- [ ] `queryParams` diteruskan ke `GeneratePagination`  
- [ ] Tidak ada hardcoded `c.Status(...).JSON(...)` di luar `pkg/response`

---

## 5. Logging (`pkg/logger`)
Setiap method service wajib log. Level:
| Operasi | Level |
|---|---|
| Read sukses / list | `Debug` |
| Write sukses (create/update/delete) | `Info` |
| Not found / validation fail / unauthorized | `Warn` |
| DB/external/unexpected error | `Error |

✅ `s.log.Info(ctx, "user created", "user_id", id, "email", email)`  
❌ `log.Println(...)`, `fmt.Printf(...)`, atau tanpa log  

**Checklist:**
- [ ] Setiap service method punya minimal 1 log  
- [ ] Structured key-value (bukan string format)  
- [ ] Error log include `"error", err`  
- [ ] Level sesuai tabel  
- [ ] Tidak ada logging di handler/repository (kecuali retry/backoff)  
- [ ] Context (`ctx`) selalu diteruskan  

---

## 6. Database — GORM Patterns
### AutoMigrate
✅ Satu model per call, error jelas  
❌ `db.AutoMigrate(&A{}, &B{}, &C{})` — tidak tahu mana error  

### FK Dependency Order (`migrate.go`)
```
auth → zone → vehicle → rfid → fee → transaction → payment → override → ocr → audit → zone_capacity_logs → gate_pairing_codes → notifications
```
→ Model baru setelah semua FK dependency-nya.

### Constraint & Index
Manual constraint (unique/composite/`CHECK`) harus di `applyManualConstraints()` di `migrate.go`.  
→ Update struct **dan** SQL saat ubah schema.

### Query Patterns
✅ `r.db.WithContext(ctx).Preload("Role.Permissions").First(...)`  
✅ `Where("deleted_at IS NULL")` untuk soft delete  
✅ `Clauses(clause.OnConflict{DoNothing: true}).Create(...)`  
❌ Tanpa `.WithContext(ctx)`  
❌ N+1: loop query dalam loop  

**Checklist:**
- [ ] Semua query pakai `.WithContext(ctx)`  
- [ ] Tidak ada `db.Find()` tanpa context  
- [ ] Soft delete konsisten di `users`  
- [ ] Preload hanya relasi yang dipakai  
- [ ] Tidak ada N+1 — gunakan batch/JOIN  
- [ ] Field baru punya `column:` tag  

---

## 7. Middleware & Auth
### Route Order (Fiber)
Static sebelum parametric dalam grup:
```go
group.Get("/", h.list)
group.Get("/code/:code", h.getByCode)  // ✅ static dulu
group.Get("/:id", h.getByID)           // parametric terakhir
```
❌ `/`:id dulu → swallow `/code/:code`

### Token Extraction
- `middleware.GetUserID(c)`, `GetUserRole(c)`, `GetClaims(c)`  
- `GateAuth`: hanya Bearer header (tidak cookie/query)  
- `AuthSSE`: fallback ke `?token=`  

### Gate JWT Secret
✅ `cfg.GateJWTSecret()`  
❌ `cfg.JWT.SecretKey`

### Session Cache Invalidation
- `Logout` → `sessionCache.Delete(hashToken(token))` + `RevokeBySessionID`  
- `RevokeAll`/password change → `userRevokedAt.Store(userID, now)`  
→ Jangan bypass — token revoked bisa dipakai 60s  

---

## 8. Validator
✅ `if errs := h.v.Validate(req); errs != nil { return response.BadRequest(...) }`  
❌ Validasi manual di handler  

**Tag umum:**
```go
validate:"required"
validate:"required,min=8"
validate:"required,email"
validate:"omitempty,uuid4"
validate:"required,oneof=motor mobil"
```
Field opsional: `omitempty` di depan  
→ `Name *string `json:"name" validate:"omitempty,min=2,max=100"`  

---

## 9. Cross-Module Dependency
Tidak boleh import domain struct modul lain untuk write. Gunakan interface inject.

✅ OCR → Transaction via `TransactionStamperPort`  
❌ OCR import `txDomain.Transaction{}` langsung  

**Allowed deps:**
```
auth         → none
zone         → none
vehicle      → none
rfid         → vehicle (read)
fee          → zone, vehicle (read)
transaction  → zone, vehicle, rfid, fee, ocr (read via interface)
payment      → transaction (write via ServicePort), zone
gate         → zone (read)
ocr          → transaction (write via StamperPort)
override     → standalone
audit        → standalone
kiosk        → fee, vehicle, zone (read)
dashboard    → DB direct (aggregate — ok)
```

---

## 10. Config & Environment
Init di `main()`:
```go
config.LoadEnv(".env")
cfg, err := config.Load()
```

**Wajib (panic jika tidak ada):**
- `JWT_SECRET_KEY` ≥32 chars  
- `QR_SECRET` (HMAC filename tiket)  

**Penting:**
- `JWT_GATE_SECRET_KEY` (fallback ke `JWT_SECRET_KEY`)  
- `ALLOW_ORIGINS` ≠ `"*"` di prod  
- `S3_*`, `MIDTRANS_SERVER_KEY`  

**Checklist:**
- [ ] Tidak ada hardcoded secret/key  
- [ ] Tidak ada `os.Getenv()` di luar `pkg/config`  
- [ ] `cfg.GateJWTSecret()` dipakai untuk gate  
- [ ] `ALLOW_ORIGINS != "*"` di production  

---

## 11. Photo Upload (`pkg/photo`)
```go
publicURL, ocrPath, err := photo.Save(fileHeader, "entry", cfg.S3)
```
- `publicURL`: S3 publik (response & OCR fetch)  
- `ocrPath`: = `publicURL` (OCR via HTTP, bukan volume)  
- `EnsureBucketPolicy(cfg.S3)` di `bootstrap/app.go`  
- HTTP client timeout 30s — jangan ganti ke `DefaultClient`  

---

## 12. Payment & Webhook
Webhook Midtrans wajib:
1. Log raw body ke `midtrans_callbacks`  
2. Verifikasi signature **sebelum** proses  
3. Gunakan `MarkPaidAndExited` (atomic)  

✅ `txSvc.MarkPaidAndExited(ctx, txID, ...)`  
❌ `MarkPaid` + `MarkExited` terpisah  

QRIS dev: kiosk poll `GET /gate/payments/:id/poll` tiap 3s (webhook tidak jalan di localhost)  

---

## 13. Simulate Endpoint — Guard Wajib
```go
if cfg.Midtrans.Env == "sandbox" {
    gateTx.Patch("/:id/simulate", h.simulate)
}
```
Dan di service:
```go
if s.cfg.Midtrans.Env != "sandbox" {
    return nil, errors.New(errors.ErrForbidden, "only in sandbox")
}
```
→ Harus guarded di route + service  

---

## 14. Debt Tracker
Cek apakah temuan termasuk debt diketahui:
| ID | Masalah | Status |
|---|---|---|
| TD-01 | `container.go` god object (40+ field) | Pending |
| TD-02 | Zero test coverage | Pending |
| TD-03 | Cashier SSE → polling (Cloudflare) | Pending |
| TD-04 | QRIS sandbox simulator manual | Pending |

Temuan baru → tambahkan ke `debt.md`  

---

## 15. Checklist Per Module Baru
### `domain.go`
- [ ] `column:` semua field  
- [ ] `TableName()` tiap struct  
- [ ] FK: `uuid.UUID` + `type:uuid` + index  
- [ ] Timestamp: `autoCreateTime`/`Update`  
- [ ] Date-only: `types.DateOnly`  
- [ ] Soft delete: `users` saja pakai `deleted_at`  

### `ports.go`
- [ ] `RepoPort` = DB only  
- [ ] `ServicePort` = logic only  
- [ ] Comment jelas, no Fiber/HTTP type  

### `repository.go`
- [ ] `.WithContext(ctx)` semua query  
- [ ] Return `errors.FromDB(err, ...)` — bukan raw GORM  
- [ ] `FindByID` preload relasi yang dipakai  
- [ ] List: offset pagination + `Count` terpisah  
- [ ] Tidak ada logic bisnis  

### `service.go`
- [ ] Log tiap method (level sesuai §5)  
- [ ] Return `errors.New(...)` — bukan raw err  
- [ ] Tidak ada `*fiber.Ctx` di param  
- [ ] Tidak ada `c.JSON(...)` / Fiber dep  
- [ ] Cross-module write via interface  

### `dto.go`
- [ ] Request: `json:` + `validate:`  
- [ ] Response: `json:`  
- [ ] `json:"-"` untuk sensitif (token/hash)  
- [ ] Pointer untuk optional update (`*string`)  
- [ ] `toResponse()` converter ada & dipakai  

### `handler.go`
- [ ] Parse → validate → service → response  
- [ ] UUID parse via `uuid.Parse()` + error handle  
- [ ] Body via `c.BodyParser()` + error handle  
- [ ] Tidak ada logic bisnis / GORM / DB  
- [ ] `middleware.GetUserID(c)` untuk actor  

### `routes.go`
- [ ] Static sebelum parametric  
- [ ] Middleware `Auth`/`RequireRole`/`RequirePermission` pasang sesuai  
- [ ] Gate route pakai `GateAuth`/`GateAuthSSE`  
- [ ] Simulate route dibungkus `cfg.Env == "sandbox"`  

---

## 16. Security Checklist
- [ ] Token/secret tidak di-log  
- [ ] Raw JWT tidak disimpan ke DB (hash SHA-256 dulu)  
- [ ] Password tidak di-log  
- [ ] `LoginResponse` tidak ekspos `Token`/`RefreshToken` (`json:"-"`)  
- [ ] Token via httpOnly cookie — bukan response body  
- [ ] `refresh_token` cookie path = `/api/v1/auth/refresh`  
- [ ] CORS `AllowCredentials=true` hanya jika `AllowOrigins != "*"`  
- [ ] Webhook Midtrans verifikasi signature dulu  
- [ ] Password: min 8 char, huruf+angka  
- [ ] Account lockout setelah 5 gagal login — tidak bypass  

---

## 17. Performance Checklist
- [ ] Tidak ada N+1 di list endpoint  
- [ ] `sessionCache` (sync.Map TTL 60s) dipakai — tidak DB lookup per ValidateToken  
- [ ] `Preload` hanya relasi yang dirender  
- [ ] Index di semua FK & kolom filter/sort  
- [ ] `DeleteExpired` dipanggil berkala  
- [ ] HTTP client punya timeout (photo/Midtrans/OCR)  
- [ ] Goroutine punya exit condition — tidak leak  

---