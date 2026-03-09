# Go-Api Codebase Audit Report
> parkieee / Projek UKK — generated 2026-03-08

---

## Summary

| | |
|---|---|
| Files scanned | ~45 (semua module, pkg, bootstrap, database) |
| Critical findings | 4 |
| Warnings | 9 |
| Suggestions | 6 |
| Test coverage | **0%** — tidak ada satu pun `_test.go` |

---

## Critical (act on these first)

### `internal/bootstrap/server.go:33` — NO PANIC RECOVERY

- **Issue:** `app.Use(recover.New())` di-comment out. Kalau ada `nil pointer dereference` atau panic lain di handler, server **langsung crash** — semua koneksi mati seketika.
- **Risk:** Denial of Service. Satu request yang bikin panic → semua user di-drop.
- **Proposed fix:** Uncomment baris itu. Fiber `recover` middleware sudah ada di go.mod. Minimal tambah custom recover yang log panic sebelum kasih 500.

---

### `internal/modules/transaction/routes.go:19` — SIMULATE ENDPOINT EXPOSED DI PRODUCTION

- **Issue:** `gateTx.Patch("/:id/simulate", adapter.h.simulate)` bisa diakses oleh siapapun yang punya gate token yang valid — termasuk di environment production. Endpoint ini memundurkan `entry_at` sampai berjam-jam, yang berarti manipulasi perhitungan tarif.
- **Risk:** Fraud. Kendaraan bayar tarif minimum dengan memundurkan waktu masuk.
- **Proposed fix:**
  ```go
  // Dalam service.SimulateEntryTime, tambahkan guard:
  if !s.isDevelopment {
      return nil, errors.New(errors.ErrForbidden, "simulate is only available in development")
  }
  ```
  Atau daftarkan route-nya hanya kalau `!container.Config.IsProduction()`.

---

### `internal/modules/ocr/service.go:176` — CROSS-MODULE DB WRITE

- **Issue:** OCR service langsung menulis ke tabel `transactions` via raw GORM call di dalam `createReviewLog`, tanpa melewati `transaction.ServicePort`:
  ```go
  s.db.WithContext(ctx).Model(...).Table("transactions").Update("plate_mismatch", mismatch)
  ```
  Ini bypass semua business logic, audit log, dan validasi yang ada di transaction service.
- **Risk:** Data inconsistency. `plate_mismatch` bisa diset bahkan kalau transaksi sudah selesai/cancelled. Tidak ada log perubahan di `transaction_logs`.
- **Proposed fix:** Tambahkan method `StampPlateMismatch(ctx, txID, mismatch bool) error` ke `transaction.ServicePort` dan inject ke OCR service, atau emit event yang dihandle transaction service.

---

### `pkg/photo/photo.go:87` — HTTP CLIENT TANPA TIMEOUT

- **Issue:** `http.DefaultClient.Do(req)` dipakai untuk upload ke S3. `http.DefaultClient` tidak punya timeout. Upload foto besar ke endpoint S3 yang lambat/mati akan **block goroutine selamanya**.
- **Risk:** Goroutine leak. Upload foto dipanggil via `go s.ocrSvc.DispatchOCRJob(...)` dan di `photo.Save` — kalau S3 tidak respond, goroutine itu gantung sampai proses mati.
- **Proposed fix:**
  ```go
  var httpClient = &http.Client{Timeout: 30 * time.Second}
  // ganti http.DefaultClient.Do(req) dengan httpClient.Do(req)
  ```

---

## Warnings

### `internal/bootstrap/server.go:39` — WILDCARD CORS

- **Issue:** `AllowOrigins: "*"` berlaku untuk semua environment termasuk production.
- **Suggestion:** Di production, set ke domain yang diketahui, misalnya `AllowOrigins: container.Config.App.URL`. Bisa conditional berdasarkan `IsProduction()`.

---

### `internal/modules/gate/service.go` + `internal/modules/auth/service.go` — SHARED JWT SECRET

- **Issue:** User JWT dan gate JWT di-sign dengan kunci yang sama (`s.cfg.JWT.SecretKey`). Satu-satunya pembeda adalah claim `"kind": "gate"`. Kalau ada bug di `ValidateGateToken` yang lupa cek `kind`, gate token bisa masquerade sebagai user token atau sebaliknya.
- **Suggestion:** Gunakan secret terpisah untuk gate token (`JWT_GATE_SECRET_KEY`), atau tambahkan audience claim (`aud`) yang berbeda dan validasi ketat di kedua parser.

---

### `database/database.go:21` vs `pkg/config/config.go:195` — TIMEZONE INKONSISTEN

- **Issue:** `Config.DSN()` menggunakan `TimeZone=Asia/Jakarta`, tapi `database.New()` membangun DSN-nya sendiri dengan `TimeZone=UTC`. **`Config.DSN()` tidak dipakai di mana pun** — yang aktual dipakai adalah yang di `database.New()`. Waktu di DB disimpan dalam UTC, tapi semua kode asumsinya WIB.
- **Suggestion:** Hapus `Config.DSN()`, atau gunakan itu dari `database.New()`. Tentukan satu timezone dan konsisten. Untuk parking system WIB, UTC di DB + konversi di layer response adalah pattern yang lebih aman.

---

### `internal/modules/transaction/service.go:160` — WRONG EVENT TYPE PADA CANCEL

- **Issue:** Method `Cancel` menggunakan `types.EventExitRecorded` sebagai event log untuk pembatalan transaksi. Ini semantiknya salah — cancel bukan exit.
- **Suggestion:** Tambahkan konstanta `types.EventCancelled` dan gunakan itu. Kalau sudah ada, pakai yang benar.

---

### `internal/modules/payment/service.go:68–85` — NON-ATOMIC MARKPAID + MARKEXITED

- **Issue:** `PayCash` memanggil `MarkPaid` lalu `MarkExited` secara berurutan tapi tidak dalam satu database transaction. Kalau `MarkPaid` sukses tapi `MarkExited` gagal (koneksi DB putus, dsb), transaksi nyangkut di status `paid` tanpa pernah jadi `exited`.
- **Suggestion:** Wrap keduanya dalam satu `s.db.Transaction(...)`, atau buat single `MarkPaidAndExited` method di `transaction.ServicePort`.

---

### `internal/modules/auth/service.go:58` — SESSION DB LOOKUP SETIAP REQUEST

- **Issue:** `ValidateToken` melakukan query ke DB (`sessionRepo.FindByTokenHash`) untuk setiap request yang membutuhkan autentikasi. Dengan 25 max open conns dan traffic tinggi, ini bisa jadi bottleneck.
- **Suggestion:** Untuk jangka pendek: tambahkan in-memory TTL cache (misalnya `sync.Map` dengan expiry sederhana) untuk token yang baru divalidasi. Untuk jangka panjang: Redis session store.

---

### `internal/modules/auth/service.go` — TIDAK ADA VALIDASI KOMPLEKSITAS PASSWORD

- **Issue:** `CreateUser` dan `ChangePassword` menerima password apapun, termasuk `"1"` atau `""` (validator struct tag tidak terlihat menolak ini dari context yang dibaca).
- **Suggestion:** Tambahkan validator custom: minimal 8 karakter, kombinasi huruf + angka. Bisa lewat `go-playground/validator` custom tag.

---

### `internal/modules/gate/repository.go` (via pairing confirm) — GATE JWT DISIMPAN PLAINTEXT DI DB

- **Issue:** Gate JWT disimpan ke kolom `gate_pairing_codes` via `pairingRepo.Confirm`. JWT ini berlaku 365 hari. Kalau DB bocor, semua gate JWT yang pernah di-pairing bisa dipakai langsung.
- **Suggestion:** Simpan hash SHA-256 dari JWT, bukan JWT itu sendiri. Saat SSE push, kirim plaintext JWT hanya ke channel — jangan simpan ke DB. Atau set TTL pendek untuk gate JWT dan refresh otomatis.

---

## Refactor Candidates

### `internal/bootstrap/container.go` — GOD OBJECT / SERVICE LOCATOR

- **Current structure:** `Container` punya 40+ field (repos + services semua level). Setiap modul baru nambah field di sini. Testing sangat susah — untuk test satu handler, harus setup seluruh container.
- **Proposed structure:**
  ```
  internal/bootstrap/
  ├── container.go       (cukup DB, Config, Log, Validator)
  └── modules/
      ├── auth_module.go      (struct AuthModule { Repo, Service })
      ├── zone_module.go
      ├── transaction_module.go
      └── ...
  ```
  Setiap module struct punya method `Register(router)` sendiri, terima hanya dependency yang dia butuhkan.
- **Dependencies to untangle:** `TransactionService` paling banyak dependency (9 service/repo). Bisa diekstrak ke `TransactionDependencies` struct.
- **Tests required before proceeding:** Yes — minimal integration test untuk route registration.

---

### `internal/modules/transaction/service.go` + `internal/modules/zone/service.go` — DUPLIKASI CAPACITY LOGIC

- **Current structure:** `appendCapacityLog` di `transaction/service.go` (private function, ~30 baris) menduplikasi logika yang juga ada di `zone.RecordCapacityEvent`. Dua implementasi berbeda untuk hal yang sama.
- **Proposed structure:** Hapus `appendCapacityLog` dari transaction service. Inject `zone.ServicePort` yang sudah ada dan panggil `RecordCapacityEvent` dalam transaction database transaction (pass `*gorm.DB` lewat parameter atau refactor port-nya).
- **Dependencies to untangle:** Transaction service sudah inject `zoneSvc` — tinggal extend port-nya.
- **Tests required before proceeding:** Yes — unit test untuk `RecordCapacityEvent`.

---

### `internal/modules/transaction/http_adapter.go` — WRAPPER YANG TIDAK BERGUNA

- **Current structure:** `httpAdapter` hanya wraps `*handler` dengan satu field. Tidak ada logic di adapter. Semua logic di handler langsung.
- **Proposed structure:** Hapus `httpAdapter`, gunakan `handler` langsung di `routes.go`. Ini menghilangkan satu layer indirection tanpa kehilangan apapun.
- **Tests required before proceeding:** No.

---

## Test Coverage Gaps

**Tidak ada satu pun file `_test.go` di seluruh codebase.** Ini adalah gap terbesar.

Prioritas test yang harus ditulis sebelum refactor apapun:

| Package | Test yang disarankan |
|---|---|
| `pkg/errors` | Unit test `FromDB`, `New`, `Wrap`, `statusFromCode` |
| `internal/modules/fee/service.go` | Unit test `calculateTierFee` dan `CalculateFee` — pure logic, mudah di-test tanpa DB |
| `internal/modules/auth/service.go` | Unit test `generateJWT`, `parseJWT`, `hashToken`, `recordFailedAttempt` |
| `internal/modules/transaction/service.go` | Integration test `RecordEntry` dan `RecordExit` dengan DB in-memory atau testcontainers |
| `pkg/photo/photo.go` | Unit test `signedPut` dengan mock HTTP server |
| `internal/modules/payment/service.go` | Unit test `verifyMidtransSignature` |

---

## Quick Wins (safe, low-risk, bisa langsung dikerjakan)

1. **Uncomment `recover.New()`** di `server.go` — 1 baris, zero risk, langsung lindungi dari crash.
2. **Tambahkan timeout ke HTTP client di `photo.go`** — 3 baris, eliminasi goroutine leak.
3. **Guard `simulate` route dengan env check** — 3 baris di routes atau service.
4. **Hapus `Config.DSN()`** yang tidak dipakai dari `config.go` — dead code cleanup.
5. **Hapus comment `//\"parkieee/pkg/logger\"`** dari `bootstrap/database.go` — dead import.
6. **Ganti `types.EventExitRecorded` → `types.EventCancelled`** di `service.Cancel` — bug fix semantik.
7. **Hapus `min()` function** di `fee/service.go` — Go 1.21+ sudah punya builtin `min`, codebase pakai Go 1.25.
8. **Hapus `httpAdapter` wrapper** di transaction module — 1 file dihapus, simplifikasi tanpa tradeoff.

---

*Scan phase selesai. Tidak ada file yang dimodifikasi. Konfirmasi sebelum tindakan apapun.*
