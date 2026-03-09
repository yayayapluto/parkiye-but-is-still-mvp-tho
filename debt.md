# Tech Debt

> Terakhir diupdate: 2026-03-09
> Semua item di bawah disengaja ditunda — bukan lupa, tapi scope-nya terlalu besar untuk dikerjakan atomik.

---

## TD-01 — God Object `container.go`

**File:** `internal/bootstrap/container.go`

**Masalah:**
`Container` punya 40+ field yang mencampur semua repo dan service dari seluruh modul. Setiap modul baru nambah field di sini. Tidak ada batasan dependency antar modul — siapapun bisa akses field apapun lewat container. Testing satu handler berarti harus setup seluruh container.

**Contoh konkret saat ini:**
```
Container
├── AuthUserRepo, AuthRoleRepo, AuthPermRepo, AuthRolePermRepo ...  (7 field auth)
├── ZoneRepo, GateRepo, GateDeviceRepo, CapacityLogRepo ...         (5 field zone)
├── TransactionRepo, TransactionLogRepo, TransactionService ...     (3 field tx)
└── ... total 40+ field
```

**Target struktur:**
```
internal/bootstrap/
├── container.go          — hanya DB, Config, Log, Validator
└── modules/
    ├── auth_module.go    — struct AuthModule { repos..., Service }; method Register(router)
    ├── zone_module.go
    ├── transaction_module.go
    └── ...
```

Setiap module struct:
- Punya field sendiri (repos + service)
- Punya method `Register(router fiber.Router)` untuk daftarkan routes
- Terima hanya dependency yang dia butuhkan — bukan seluruh container

**Kenapa ditunda:**
Audit mensyaratkan integration test route registration sebelum refactor ini aman. Refactor tanpa test berisiko merusak wiring dependency yang subtle (terutama circular dep OCR → Transaction yang sudah di-resolve manual di `NewContainer`).

**Prerequisites sebelum mulai:**
1. Integration test minimal untuk `server.go` yang verifikasi semua route terdaftar
2. Test yang verifikasi circular dep OCR → Transaction tetap ter-wire setelah refactor

**Estimasi effort:** L (2–3 hari)

---

## TD-02 — Transaction Integration Test

**File:** `internal/modules/transaction/service.go`

**Masalah:**
`RecordEntry` dan `RecordExit` adalah operasi paling kritikal di sistem — melibatkan banyak dependency (zone, rfid, fee, vehicle, ocr) dan menulis ke beberapa tabel dalam satu DB transaction. Saat ini tidak ada test apapun yang meng-cover happy path maupun failure path dari kedua method ini.

**Risk tanpa test ini:**
- Refactor capacity logic, fee calculation, atau RFID validation bisa silent-break
- Tidak ada safety net kalau ada regression di RecordEntry/RecordExit

**Kenapa ditunda:**
Butuh database nyata atau testcontainers. Mock-based test tidak cukup karena behavior kritis ada di multi-table DB transaction — kalau satu write gagal, semua harus rollback. Ini tidak bisa diverifikasi dengan mock.

**Rencana implementasi:**
```
// Setup testcontainers PostgreSQL
func setupTestDB(t *testing.T) *gorm.DB { ... }

// Test cases minimal:
TestRecordEntry_HappyPath_RFID
TestRecordEntry_HappyPath_QR
TestRecordEntry_ZoneFull_Rejected
TestRecordEntry_RFIDAlreadyOpen_Rejected
TestRecordExit_HappyPath
TestRecordExit_WrongGate_Rejected
TestRecordExit_RFIDMismatch_Rejected
TestCancel_ReleasesCapacity
```

**Dependencies yang perlu di-setup:**
- `testcontainers-go` + PostgreSQL image
- Database migration runner (atau GORM AutoMigrate di test setup)
- Seed data: zone, gate entry, gate exit, vehicle type, fee config

**Estimasi effort:** XL (3–5 hari termasuk setup testcontainers)

---

## Status Summary

| ID | Judul | Effort | Blocker |
|---|---|---|---|
| TD-01 | God Object container.go | L | Integration test route registration belum ada |
| TD-02 | Transaction integration test | XL | Butuh testcontainers setup |

**Urutan yang disarankan:** TD-02 dulu → hasil test-nya sekaligus jadi enabler untuk TD-01.
