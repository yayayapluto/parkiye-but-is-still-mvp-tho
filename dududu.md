---

**Step 1 — `pkg/types/`**
Isi enum dan constants dari ERD. Dibutuhkan semua `domain.go`.
```
transaction.go → TransactionStatus, EntryMethod, ExitMethod
payment.go     → PaymentMethod, PaymentStatus
gate.go        → GateType
```

---

**Step 2 — Semua `domain.go` sekaligus**
Struct GORM mapping 1:1 ke ERD, semua module sekaligus karena ada cross-FK. Urutan penulisan ikut dependency FK di ERD:
```
auth → zone → gate → vehicle → rfid → fee → transaction → payment → override → ocr → audit
```

---

**Step 3 — Migrations**
SQL di `database/migrations/` berdasarkan domain. Urutan sama dengan step 2 karena FK constraint.

**Checkpoint:** `go run` + migrate → semua tabel terbuat di DB.

---

**Step 4 — `pkg/` infrastructure**
```
config → logger → tracer → metrics → errors → response → validator → helpers
```
Urutan ini karena logger butuh config, tracer butuh config, dst.

---

**Step 5 — Bootstrap**
```
database.go → observability.go → container.go (kosong dulu) → server.go → app.go
```

**Checkpoint:** App bisa start, connect DB, `/health` endpoint response.

---

**Step 6 — Module per Phase**
Per module, urutan file selalu sama:
```
ports.go → repository.go → service.go → dto.go → handler.go → routes.go
```
Urutan phase:
```
audit → auth → zone → gate → vehicle → rfid → fee → transaction → payment → override → ocr
```

---

**Step 7 — Wire `container.go`**
Setelah semua module selesai. Kalau compile error di sini berarti ada port yang belum diimplementasikan.

---

**Step 8 — `docker-compose.yml`**
PostgreSQL aktif. Observability stack (Prometheus, Loki, Grafana, Tempo) di-comment untuk MVP.

---