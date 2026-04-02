# Parkieee — All Business Logic

Dokumen ini merangkum seluruh logika bisnis yang sudah diimplementasi di Go API.
Ditulis agar mudah dibaca manusia — bukan referensi kode, tapi penjelasan cara kerja sistem.

Terakhir diupdate: 2026-03-19

---

## Daftar Isi

1. [Cara Sistem Mengenali Siapa yang Request](#1-cara-sistem-mengenali-siapa-yang-request)
2. [Login dan Keamanan Akun](#2-login-dan-keamanan-akun)
3. [Alur Masuk Kendaraan (Entry)](#3-alur-masuk-kendaraan-entry)
4. [Alur Keluar Kendaraan (Exit)](#4-alur-keluar-kendaraan-exit)
5. [Perhitungan Tarif Parkir](#5-perhitungan-tarif-parkir)
6. [Pembayaran](#6-pembayaran)
7. [Notifikasi Kasir dan Kiosk (SSE & Polling)](#7-notifikasi-kasir-dan-kiosk-sse--polling)
8. [Kasir Assignment per Exit Gate](#8-kasir-assignment-per-exit-gate)
9. [Gate Mode: manless vs with_cashier](#9-gate-mode-manless-vs-with_cashier)
10. [Pairing Gate ke Kiosk Screen](#10-pairing-gate-ke-kiosk-screen)
11. [OCR Plat Nomor](#11-ocr-plat-nomor)
12. [Kapasitas Zona](#12-kapasitas-zona)
13. [RFID](#13-rfid)
14. [Override Operator (planned)](#14-override-operator-planned)
15. [Status Transaksi — Lifecycle](#15-status-transaksi--lifecycle)
16. [Format Standar Response API](#16-format-standar-response-api)
17. [Konvensi Error](#17-konvensi-error)
18. [Seed Data dan Default](#18-seed-data-dan-default)

---

## 1. Cara Sistem Mengenali Siapa yang Request

Ada dua jenis "identitas" yang bisa masuk ke API:

### User biasa (operator, admin, kasir, dll)

Saat login berhasil, sistem generate JWT. Token ini berisi: user ID, email, role, dan daftar permission yang dimiliki role itu. Setiap request berikutnya wajib kirim token ini di header `Authorization: Bearer ...`.

Saat token diterima, sistem tidak langsung tanya database — ada cache in-memory (60 detik). Kalau token ada di cache dan belum expired, langsung dipakai. Kalau cache miss, baru cek ke database.

Cara revoke token secara instan (misalnya setelah ganti password atau nonaktifkan user): sistem simpan timestamp revoke per user. Setiap kali ada token di cache, dicek dulu apakah ada revoke yang lebih baru. Kalau ya, token langsung ditolak meski masih di cache.

### Screen gate (kiosk)

Screen gate punya token sendiri yang berbeda — ini JWT khusus gate, berisi gate ID, tipe gate (entry/exit), zone ID, nama gate, dan **mode gate** (`manless` atau `with_cashier`). Token ini berlaku 365 hari. Cara mendapatkannya: lewat proses pairing (dijelaskan di bagian 10).

Middleware `GateAuth` yang cek token ini — terpisah dari middleware user biasa.

### Permission

Setiap endpoint yang butuh akses khusus diberi middleware `RequirePermission("nama.permission")`. Permission dicek dari token (tidak hit database). Daftar permission lengkap:

| Group | Permission | Keterangan |
|---|---|---|
| Gate | gate.manage | CRUD gate |
| | gate.pair | pairing kiosk |
| | gate.view | lihat gate |
| | gate.override | override kondisi darurat |
| Zone | zone.manage | CRUD zone |
| | zone.view | lihat zone |
| Fee | fee.edit | ubah konfigurasi tarif |
| | fee.view | lihat tarif |
| User | user.manage | CRUD user |
| | user.view | lihat user |
| | cashier.assign | assign kasir ke gate |
| Payment | payment.cash | proses pembayaran tunai |
| | payment.qris | inisiasi QRIS |
| | payment.refund | kelola refund |
| Transaction | transaction.view | lihat transaksi |
| | transaction.cancel | batalkan transaksi |
| Override | override.perform | buat override |
| | override.config | kelola konfigurasi override |
| RFID | rfid.manage | kelola kartu RFID |
| | rfid.view | lihat kartu RFID |
| Report | report.view | lihat laporan |
| Audit | audit.read | lihat audit log |
| Config | config.edit | kelola konfigurasi sistem |
| Internal | internal.access | dipakai Python service ke Go API |
| Kasir | cashier.ability | akses panel kasir |

---

## 2. Login dan Keamanan Akun

### Urutan saat login

1. Cari user berdasarkan email. Kalau tidak ada, return error **generic** — sistem sengaja tidak bilang "user tidak ditemukan" supaya tidak bisa di-enumerate.
2. Cek apakah akun aktif dan tidak dihapus.
3. Cek apakah akun sedang dikunci (locked). Pengecekan ini dilakukan **sebelum** verifikasi password.
4. Verifikasi password dengan bcrypt.
5. Kalau salah → catat kegagalan, tambah counter. Kalau counter 5 kali gagal → akun dikunci.
6. Kalau benar → reset counter, buat session baru, generate JWT, return token.

Token yang dihasilkan **tidak pernah disimpan ke database secara utuh** — hanya hash SHA-256-nya.

### Lockout

Akun dikunci setelah 5 kali gagal login. Hanya admin yang bisa buka kunci.

### Ganti password

Setelah ganti password, **semua session aktif langsung dimatikan** di semua device.

---

## 3. Alur Masuk Kendaraan (Entry)

### Yang dilakukan sistem saat kendaraan masuk

1. Validasi gate: harus bertipe **entry**, aktif, zone aktif.
2. Cek kapasitas zona: kalau penuh → tolak masuk.
3. Kalau via RFID: cek kartu aktif dan tidak punya transaksi aktif.
4. Kalau via QR tiket: generate kode transaksi `PKR-YYYYMMDD-XXXXX`, generate QR image, simpan ke S3.
5. Semua operasi dalam satu DB transaction.
6. Setelah berhasil: kapasitas zona +1, OCR dispatch async (kalau ada foto).

**Status transaksi setelah entry: `open`**

---

## 4. Alur Keluar Kendaraan (Exit)

### Yang dilakukan sistem saat kendaraan keluar

1. Cari transaksi berdasarkan ID. Status harus `open`.
2. Validasi gate: harus bertipe **exit**, aktif, **zona sama** dengan gate masuk.
3. Kalau via RFID: kartu harus **sama persis** dengan saat masuk.
4. Tentukan tipe kendaraan (prioritas): hasil OCR → input kiosk → default zona.
5. Hitung tarif.
6. Update transaksi + log. Kapasitas zona -1.
7. OCR dispatch async kalau ada foto exit.

**Status transaksi setelah exit: `awaiting_payment`**

Kapasitas berkurang saat RecordExit, bukan saat payment — slot langsung tersedia untuk kendaraan berikutnya.

---

## 5. Perhitungan Tarif Parkir

### Cara hitung

1. Durasi total dalam menit: `exitTime - entryTime`.
2. Kurangi grace period: `billable = total - grace_period`. Kalau ≤ 0 → hanya bayar `base_fee`.
3. Hitung biaya tier dari `billable_minutes`.
4. `raw_fee = base_fee + tier_fee`.
5. Terapkan holiday rate kalau ada.

### Cara kerja tier

Tier diproses berurutan, masing-masing "mengonsumsi" durasi yang tersisa:
- Tier biasa: ambil `min(sisa, durasi_tier)`. Biaya adalah flat.
- Tier terakhir (`is_last_tier = true`): `ceil(sisa / durasi)` blok × biaya per blok.

### Holiday rate

- Ada **override** fee → pakai override (rata-rata kalau lebih dari satu), abaikan multiplier.
- Hanya ada **multiplier** → rata-rata multiplier × `raw_fee`.
- Override selalu menang atas multiplier.

---

## 6. Pembayaran

### Metode tunai

1. Kiosk kirim `cash_tendered`. Validasi: `cash_tendered >= calculated_fee`.
2. Hitung kembalian.
3. Buat payment `completed`.
4. `MarkPaidAndExited` atomic.

> Catatan: `MarkPaidAndExited` akan dipisah menjadi `MarkPaid` + `MarkExited` saat object detection diintegrasikan.

### Metode QRIS

1. Minta QRIS ke Midtrans. Kalau sudah ada QRIS pending → return yang lama (idempotent).
2. Kiosk polling status setiap 3 detik.
3. Midtrans kirim webhook saat user bayar.
4. Verifikasi signature: `SHA512(orderID + statusCode + grossAmount + serverKey)`.
5. Kalau valid dan status `settlement`/`capture` → `MarkPaidAndExited`.
6. Webhook selalu return HTTP 200 ke Midtrans.

### Polling kasir (cash fallback untuk Cloudflare)

Saat kiosk request tunai, catat `cashier_requested_at`. Kasir polling `GET /payments/cashier/pending?since=<ISO>` setiap 2 detik. Hanya return transaksi dari gate yang di-assign ke kasir tersebut.

### Refund

- Hanya untuk payment `completed`.
- Satu payment satu refund aktif (tidak bisa double refund).
- QRIS refund diteruskan ke Midtrans via `midtrans_transaction_id`.
- Harus di-approve admin sebelum diproses.

---

## 7. Notifikasi Kasir dan Kiosk (SSE & Polling)

### Kiosk → Kasir (request pembayaran tunai)

1. Kiosk hit `POST /gate/payments/cashier/notify`.
2. Sistem lookup kasir yang di-assign ke gate asal request dari `gate_cashier_assignments`.
3. Kirim event hanya ke SSE channel kasir dengan `userID` yang sesuai — **bukan broadcast**.
4. Kasir yang terhubung via SSE `GET /payments/cashier/listen` langsung terima.
5. Polling `GET /payments/cashier/pending` sebagai fallback (Cloudflare SSE workaround).

### Kasir → Kiosk (konfirmasi selesai)

1. Kasir hit `POST /payments/cashier/done`.
2. Sistem kirim event ke SSE channel kiosk yang menunggu untuk transaksi itu.
3. Kiosk redirect ke halaman sukses, palang dibuka.

Hanya satu kiosk listener per transaksi — koneksi lama di-close kalau ada koneksi baru untuk tx yang sama.

### Kasir online detection

Payment service simpan `cashierSeen map[uuid.UUID]time.Time` in-memory. Di-update setiap kasir hit endpoint apapun yang butuh `cashier.ability`. Kasir dianggap online kalau `now - lastSeenAt ≤ 10 detik` (dikonfigurasi via `CASHIER_ONLINE_THRESHOLD`).

---

## 8. Kasir Assignment per Exit Gate

### Konsep

Satu akun kasir = satu exit gate, strict 1:1. Kasir login biasa (email/password) — tidak ada perubahan di auth. Yang mengontrol gate mana yang dia handle adalah `gate_cashier_assignments`.

### Assignment

- Dibuat pertama kali: admin confirm pairing exit gate sekalian pilih kasir
- Reassign tanpa pairing ulang: `PUT /zones/:id/gates/:gateId/cashier` (permission: `cashier.assign`)
- Unassign: `DELETE /zones/:id/gates/:gateId/cashier`
- Kalau kasir tidak hadir → akun bisa dipakai orang lain sebagai backup
- Kalau lane tutup → hapus assignment, kiosk tampilkan "tidak beroperasi"

### SSE routing

Notif dari kiosk dikirim ke channel yang di-key by `userID` kasir, bukan broadcast. Kasir hanya terima notif dari gate-nya sendiri.

---

## 9. Gate Mode: manless vs with_cashier

### Konsep

Setiap exit gate punya mode yang bisa diubah admin kapan saja tanpa pairing ulang via `PATCH /zones/:id/gates/:gateId/mode`.

| Mode | Payment yang tersedia | Mark Exited | Fallback jika service down |
|---|---|---|---|
| `manless` | QRIS saja | object detection wajib | manual via panel override |
| `with_cashier` | QRIS + tunai | kasir konfirmasi | gate tidak beroperasi |

### Ketentuan

- `with_cashier` tapi kasir offline → kiosk tampilkan "Lane tidak beroperasi"
- `manless` tapi object detection down → transaksi stuck di `paid`, perlu manual `MarkExited` dari panel override
- Switch ke `with_cashier` wajib ada assignment kasir terlebih dahulu
- Mode masuk ke JWT gate saat pairing — kiosk tahu mode dari claims

### Kiosk flow berdasarkan mode

```
Kalau mode = manless:
  → Tampilkan QRIS saja
  → Setelah paid, tunggu object detection

Kalau mode = with_cashier:
  → Poll GET /gate/payments/cashier/status setiap 5 detik
  → Kalau online: tampilkan QRIS + tunai
  → Kalau offline: tampilkan "Lane tidak beroperasi"
```

---

## 10. Pairing Gate ke Kiosk Screen

Ada dua cara kiosk mendapatkan gate JWT:

### Cara 1: Token-first (manual)

Input `gate_token` yang sudah ada di DB → sistem generate JWT. Simpel tapi perlu token yang sudah diketahui.

### Cara 2: QR Pairing (flow utama)

1. **Screen request pairing**: `POST /gate/pairing/request`. Generate kode 6 karakter + QR code. Kode berlaku 5 menit.
2. **Screen subscribe SSE**: `GET /gate/pairing/:code/listen` — tunggu JWT.
3. **Admin scan QR** via panel web.
4. **Admin confirm pairing**: `POST /gate/pairing/:code/confirm` dengan `{ gate_id, cashier_user_id?, mode? }`.
   - Kalau gate exit + mode `with_cashier` → insert ke `gate_cashier_assignments` sekaligus.
   - Generate JWT gate (berlaku 365 hari), simpan hash ke DB.
   - Push JWT ke screen via SSE.
5. **Screen terima JWT** dan mulai beroperasi.

---

## 11. OCR Plat Nomor

### Cara kerja

OCR dijalankan **async (background)** — tidak memblokir response ke kiosk.

1. Dispatch OCR job (tulis ke `ocr_jobs`, status `queued`).
2. Background goroutine proses job.
3. Ping Python service: `GET /health`. Kalau tidak respon → `skipped`.
4. Call Python: `POST /detect-plate` dengan URL foto dari S3.
5. Retry sampai 2 kali kalau gagal. Kalau tetap gagal → `failed`.
6. Kalau berhasil: upsert vehicle, link ke transaksi. Confidence ≥ 0.80 → auto-verified.
7. Kalau foto exit: bandingkan plat exit dengan plat entry → catat `plate_mismatch`.

### Normalisasi plat

Hapus spasi, uppercase — `b 1701 sgi` == `B1701SGI`.

---

## 12. Kapasitas Zona

Kapasitas tidak disimpan sebagai satu angka yang di-update — dicatat sebagai **log append-only** di `zone_capacity_logs`. Untuk tahu kapasitas sekarang: ambil baris terbaru per zona.

Event yang mengubah kapasitas:
- Entry berhasil → occupied +1
- RecordExit tercatat → occupied -1
- Cancel transaksi → occupied -1

Nilai occupied tidak bisa negatif atau melebihi `zone.capacity` — ada clamping.

---

## 13. RFID

- **Registrasi**: kartu pertama kali ditempel → otomatis daftarkan (idempotent).
- **Linking**: kartu bisa di-link ke satu kendaraan. Unlink → set `vehicle_id = null`.
- **Guard entry**: satu kartu tidak bisa punya dua transaksi aktif (open atau awaiting_payment) sekaligus.
- **Guard exit**: kartu saat exit harus **sama persis** dengan saat masuk.

---

## 14. Override Operator (planned)

Belum diimplementasi. Rencana:

- Operator bisa lakukan override untuk kondisi darurat lewat panel web.
- Ada limit override per hari/minggu per operator (diatur di `OverrideConfig`).
- Setiap override dicatat lengkap: siapa, kapan, tipe, alasan, transaksi terdampak.

Tipe override yang direncanakan:
- `lost_card_exit` — keluar tanpa kartu
- `no_qr_exit` — keluar tanpa QR
- `fee_waive` — gratisin tarif
- `fee_adjust` — ubah tarif manual
- `force_open_gate` — paksa buka palang
- `manual_entry` — buat transaksi entry manual
- `manual_mark_exited` — manual trigger `MarkExited` untuk transaksi stuck di `paid` (kasus object detection down di mode `manless`)

---

## 15. Status Transaksi — Lifecycle

```
[open]
  ↓ RecordExit berhasil
[awaiting_payment]
  ↓ Payment selesai (QRIS webhook atau kasir konfirmasi tunai)
[paid]
  ↓ MarkExited
    - sekarang: langsung setelah paid (MarkPaidAndExited atomic)
    - rencana: pisah — via object detection (manless) atau kasir done (with_cashier)
[exited]

Dari [open] saja:
  ↓ Cancel oleh operator
[cancelled]

Dari [awaiting_payment] saja:
  ↓ Override operator
[overridden]
```

---

## 16. Format Standar Response API

```json
{
  "success": true,
  "meta": {
    "code": "OK",
    "message": "pesan"
  },
  "data": { ... }
}
```

Untuk response dengan pagination:

```json
{
  "success": true,
  "meta": { ... },
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total": 42,
    "total_pages": 5,
    "prev": null,
    "next": "http://host/api/v1/resource?page=2&page_size=10"
  }
}
```

HTTP status code: `200` sukses, `201` buat baru, `400` request invalid, `401` no token, `403` no permission, `404` not found, `409` konflik, `423` akun terkunci, `500` internal error.

---

## 17. Konvensi Error

Semua error pakai package internal `pkg/errors`. Error dari database (GORM) selalu di-wrap di repository layer.

```go
errors.New(errors.ErrNotFound, "pesan")
errors.Wrap(err, errors.ErrInternal, "context")
errors.IsCode(err, errors.ErrNotFound)
```

---

## 18. Seed Data dan Default

Seed idempotent — aman dijalankan berkali-kali.

### Akun admin default

```
Email    : admin@parkieee.local
Password : Admin@123!
```

### Role dan permission default

| Role | Permission utama |
|---|---|
| admin | semua permission |
| operator | gate.manage, gate.view, zone.view, fee.view, transaction.view, transaction.cancel, override.perform, rfid.view |
| owner | gate.view, zone.view, fee.edit, fee.view, user.view, report.view, config.edit |
| engineer | gate.view, zone.view, audit.read, config.edit |
| kasir | cashier.ability, payment.cash, payment.qris, transaction.view |

### Kode transaksi

Format: `PKR-YYYYMMDD-XXXXX` — sequential per hari. Contoh: `PKR-20260319-00001`.
