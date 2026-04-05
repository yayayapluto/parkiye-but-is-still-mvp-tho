# Backend Implementation Plan: Server-Side Search, Sort & Pagination

Go-Api — semua list endpoint harus support `search`, `sort_by`, `sort_order`, `page`, `page_size`.

---

## Current Status

| Endpoint | Pagination | Search | Sort | Action |
|----------|:----------:|:------:|:----:|--------|
| `GET /vehicles` | ✅ | ✅ | ✅ | **Done** — no changes needed |
| `GET /rfid/cards` | ✅ | ✅ | ✅ | **Done** — no changes needed |
| `GET /transactions` | ✅ | ✅ | ✅ | **Done** — no changes needed |
| `GET /audit` | ✅ | ✅ | ✅ | **Done** — no changes needed |
| `GET /auth/users` | ✅ | ❌ | ❌ | **Need search+sort** |
| `GET /fee/configs` | ✅ | ❌ | ❌ | **Need search+sort** |
| `GET /fee/holiday-rates` | ✅ | ❌ | ❌ | **Need search+sort** |
| `GET /zones` | ✅ | ❌ | ❌ | **Need search+sort** |
| `GET /zones/:id/gates` | ✅ | ❌ | ❌ | **Need search+sort** |
| `GET /gates` | ✅ | ❌ | ❌ | **Need search+sort** |
| `GET /vehicle-types` | ❌ | ❌ | ❌ | **Need full pagination upgrade** |
| `GET /auth/roles` | ❌ | ❌ | ❌ | **Need full pagination upgrade** |

---

## Task 1: Add Search + Sort to `listUsers`

**Pattern**: Sama seperti `ListVehicleFilter` di vehicle module.

### Files:

#### [MODIFY] `internal/modules/auth/dto.go`
- Add `ListUserFilter` struct:
```go
type ListUserFilter struct {
    Search    string
    SortBy    string
    SortOrder string
    RoleID    *uuid.UUID
    IsActive  *bool
}
```

#### [MODIFY] `internal/modules/auth/handler.go` → `listUsers`
- Parse `search`, `sort_by`, `sort_order` dari query params
- Build `ListUserFilter` dan pass ke service

#### [MODIFY] `internal/modules/auth/ports.go`
- `UserRepositoryPort.List()`: Change signature `(ctx, filter ListUserFilter, page, pageSize int)`
- `ServicePort.ListUsers()`: Change signature `(ctx, filter ListUserFilter, page, pageSize int)`

#### [MODIFY] `internal/modules/auth/repository.go` → `List()`
- Add `ILIKE` search pada `name`, `email`, `username`:
```go
if filter.Search != "" {
    q = q.Where("name ILIKE ? OR email ILIKE ? OR username ILIKE ?",
        "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
}
```
- Add dynamic `ORDER BY`:
```go
sortCol := "created_at"
sortOrder := "desc"
if filter.SortBy != "" { sortCol = filter.SortBy }
if strings.ToLower(filter.SortOrder) == "asc" { sortOrder = "asc" }
```

#### [MODIFY] `internal/modules/auth/service.go` → `ListUsers()`
- Update signature to match ports

---

## Task 2: Add Search + Sort to Fee Configs & Holiday Rates

### Files:

#### [MODIFY] `internal/modules/fee/dto.go`
- Add `ListFeeConfigFilter`:
```go
type ListFeeConfigFilter struct {
    Search        string
    SortBy        string
    SortOrder     string
    ZoneID        *uuid.UUID
    VehicleTypeID *uuid.UUID
}
```
- Add `ListHolidayRateFilter`:
```go
type ListHolidayRateFilter struct {
    Search    string
    SortBy    string
    SortOrder string
}
```

#### [MODIFY] `internal/modules/fee/handler.go` → `listFeeConfigs` & `listHolidayRates`
- Parse `search`, `sort_by`, `sort_order`
- Build filter structs

#### [MODIFY] `internal/modules/fee/ports.go`
- Update `FeeConfigRepositoryPort.FindAllConfigs()` and `HolidayRateRepositoryPort.FindAllRates()` signatures

#### [MODIFY] `internal/modules/fee/repository.go`
- `FindAllConfigs()`: search pada zone name / vehicle type name via join, dynamic sort
- `FindAllRates()`: search pada `name`, dynamic sort

#### [MODIFY] `internal/modules/fee/service.go`
- Update signatures

---

## Task 3: Add Search + Sort to Zones & Gates

### Files:

#### [MODIFY] `internal/modules/zone/dto.go` (or add filter structs inline)
- Add `ListZoneFilter`:
```go
type ListZoneFilter struct {
    Search    string
    SortBy    string
    SortOrder string
    Active    bool
}
```
- Add `ListGateFilter`:
```go
type ListGateFilter struct {
    Search    string
    SortBy    string
    SortOrder string
    ZoneID    *uuid.UUID
    GateType  *string
    Active    bool
}
```

#### [MODIFY] `internal/modules/zone/handler.go`
- `listZones`: Parse `search`, `sort_by`, `sort_order` → build filter
- `listGates` & `listAllGates`: Parse `search`, `sort_by`, `sort_order` → build filter

#### [MODIFY] `internal/modules/zone/ports.go`
- Update repository and service port signatures

#### [MODIFY] `internal/modules/zone/repository.go`
- `ListZones()`: search pada `name`, `description`, dynamic sort
- `ListGates()` & `ListAllGates()`: search pada `name`, `location`, dynamic sort

#### [MODIFY] `internal/modules/zone/service.go`
- Update signatures

---

## Task 4: Upgrade Vehicle Types → Paginated

**Currently**: `ListVehicleTypes()` calls `FindAll()` → returns `[]VehicleType` (no pagination).

### Files:

#### [MODIFY] `internal/modules/vehicle/ports.go`
- `VehicleTypeRepositoryPort`: Add `FindAllPaginated(ctx, search string, page, pageSize int) ([]VehicleType, int64, error)`
- `ServicePort`: Change `ListVehicleTypes(ctx) ([]VehicleType, error)` → `ListVehicleTypes(ctx, search string, page, pageSize int) ([]VehicleType, int64, error)`

#### [MODIFY] `internal/modules/vehicle/repository.go`
- Add `FindAllPaginated()`:
```go
func (r *vehicleTypeRepo) FindAllPaginated(ctx context.Context, search string, page, pageSize int) ([]VehicleType, int64, error) {
    var vts []VehicleType
    var total int64
    q := r.db.WithContext(ctx).Model(&VehicleType{})
    if search != "" {
        q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
    }
    if err := q.Count(&total).Error; err != nil { return nil, 0, err }
    offset := (page - 1) * pageSize
    if err := q.Order("name ASC").Offset(offset).Limit(pageSize).Find(&vts).Error; err != nil { return nil, 0, err }
    return vts, total, nil
}
```

#### [MODIFY] `internal/modules/vehicle/service.go`
- Update `ListVehicleTypes()` to call `FindAllPaginated()`

#### [MODIFY] `internal/modules/vehicle/handler.go`
- `listVehicleTypes`: Parse pagination + search, call service, return `response.Paginated()`

> [!IMPORTANT]
> Keep `FindAll()` method — kalau ada tempat lain yang pakai (misal dropdown di form), tetap bisa panggil tanpa pagination.

---

## Task 5: Upgrade Roles → Paginated

**Currently**: `GetAllRoles()` calls `FindAll()` → returns `[]Role` (no pagination).

### Files:

#### [MODIFY] `internal/modules/auth/ports.go`
- `RoleRepositoryPort`: Add `FindAllPaginated(ctx, search string, page, pageSize int) ([]Role, int64, error)`
- `ServicePort`: Add `ListRoles(ctx, search string, page, pageSize int) ([]Role, int64, error)` — keep `GetAllRoles()` for dropdown internal use

#### [MODIFY] `internal/modules/auth/repository.go`
- Add `FindAllPaginated()` on `roleRepository`:
```go
func (r *roleRepository) FindAllPaginated(ctx context.Context, search string, page, pageSize int) ([]Role, int64, error) {
    var roles []Role
    var total int64
    q := r.db.WithContext(ctx).Model(&Role{})
    if search != "" {
        q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
    }
    if err := q.Count(&total).Error; err != nil { return nil, 0, err }
    offset := (page - 1) * pageSize
    if err := q.Preload("Permissions.Permission").Order("name ASC").Offset(offset).Limit(pageSize).Find(&roles).Error; err != nil {
        return nil, 0, err
    }
    return roles, total, nil
}
```

#### [MODIFY] `internal/modules/auth/service.go`
- Add `ListRoles()` method

#### [MODIFY] `internal/modules/auth/handler.go`
- `getRoles` → `listRoles`: Parse pagination + search, return `response.Paginated()`

---

## Execution Order

| # | Task | Module | Files Modified | Complexity |
|---|------|--------|----------------|------------|
| 1 | Search+Sort for Users | `auth` | handler, repository, ports, service, dto | Medium |
| 2 | Search+Sort for Fees | `fee` | handler, repository, ports, service, dto | Medium |
| 3 | Search+Sort for Zones+Gates | `zone` | handler, repository, ports, service, dto | Medium |
| 4 | Paginate Vehicle Types | `vehicle` | handler, repository, ports, service | Medium |
| 5 | Paginate Roles | `auth` | handler, repository, ports, service | Medium |

---

## Verification

After each task:
```sh
go build ./...
```

After all tasks, test endpoints:
```sh
# Users — search + sort
curl "localhost:8080/api/v1/auth/users?page=1&page_size=5&search=admin&sort_by=name&sort_order=asc"

# Fee configs — search
curl "localhost:8080/api/v1/fee/configs?page=1&page_size=5&search=motor"

# Zones — search
curl "localhost:8080/api/v1/zones?page=1&page_size=5&search=basement"

# Vehicle types — now paginated
curl "localhost:8080/api/v1/vehicle-types?page=1&page_size=5&search=motor"

# Roles — now paginated
curl "localhost:8080/api/v1/auth/roles?page=1&page_size=5"
```

Verify all responses match `PaginatedResponse` shape with `pagination.meta.last_page`.
