package zone

import (
	"context"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

type ZoneRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Zone, error)
	FindAll(ctx context.Context, filter ListZoneFilter, page, pageSize int) ([]Zone, int64, error)
	Create(ctx context.Context, zone *Zone) error
	Update(ctx context.Context, zone *Zone) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type GateRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Gate, error)
	FindByZoneID(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error)
	FindAll(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error)
	FindByToken(ctx context.Context, token string) (*Gate, error)
	Create(ctx context.Context, gate *Gate) error
	Update(ctx context.Context, gate *Gate) error
	UpdateTokenLastUsed(ctx context.Context, id uuid.UUID) error
	UpdateMode(ctx context.Context, id uuid.UUID, mode types.GateMode) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type GateCashierAssignmentRepositoryPort interface {
	// Upsert inserts or replaces the assignment for a gate (UNIQUE on gate_id).
	Upsert(ctx context.Context, a *GateCashierAssignment) error
	// FindByGateID returns the current assignment for a gate, ErrNotFound if none.
	FindByGateID(ctx context.Context, gateID uuid.UUID) (*GateCashierAssignment, error)
	// FindByUserID returns the gate currently assigned to a user, ErrNotFound if none.
	FindByUserID(ctx context.Context, userID uuid.UUID) (*GateCashierAssignment, error)
	// DeleteByGateID removes the assignment for a gate.
	DeleteByGateID(ctx context.Context, gateID uuid.UUID) error
}

type GateDeviceRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*GateDevice, error)
	FindByGateID(ctx context.Context, gateID uuid.UUID) ([]GateDevice, error)
	Upsert(ctx context.Context, device *GateDevice) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CapacityLogRepositoryPort interface {
	Append(ctx context.Context, log *ZoneCapacityLog) error
	LatestByZoneID(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityLog, error)
	// AllCapacities returns the latest capacity snapshot for every active zone
	// in a single aggregated query (DISTINCT ON zone_id ORDER BY recorded_at DESC).
	// Zones with no log entry yet are included with occupied=0, available=capacity.
	AllCapacities(ctx context.Context) ([]ZoneCapacityResponse, error)
}

type ServicePort interface {
	// Zone management
	GetZone(ctx context.Context, id uuid.UUID) (*Zone, error)
	ListZones(ctx context.Context, filter ListZoneFilter, page, pageSize int) ([]Zone, int64, error)
	CreateZone(ctx context.Context, req *CreateZoneRequest, actorID uuid.UUID) (*Zone, error)
	UpdateZone(ctx context.Context, id uuid.UUID, req *UpdateZoneRequest) (*Zone, error)
	DeactivateZone(ctx context.Context, id uuid.UUID) error

	// Gate management
	GetGate(ctx context.Context, id uuid.UUID) (*Gate, error)
	ListGates(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error)
	CreateGate(ctx context.Context, req *CreateGateRequest, actorID uuid.UUID) (*Gate, error)
	UpdateGate(ctx context.Context, id uuid.UUID, req *UpdateGateRequest) (*Gate, error)
	DeactivateGate(ctx context.Context, id uuid.UUID) error
	RegenerateGateToken(ctx context.Context, id uuid.UUID) (*Gate, error)
	// UpdateGateMode changes the operating mode of an exit gate.
	// Validates that a cashier assignment exists before switching to with_cashier.
	UpdateGateMode(ctx context.Context, id uuid.UUID, mode types.GateMode, actorID uuid.UUID) (*Gate, error)
	ListAllGates(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error)

	// Cashier assignment
	AssignCashier(ctx context.Context, gateID, userID, assignedBy uuid.UUID) (*GateCashierAssignment, error)
	UnassignCashier(ctx context.Context, gateID uuid.UUID) error
	GetCashierAssignment(ctx context.Context, gateID uuid.UUID) (*GateCashierAssignment, error)
	GetAssignmentByUser(ctx context.Context, userID uuid.UUID) (*GateCashierAssignment, error)

	// Capacity
	GetCapacity(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityResponse, error)
	// ListAllCapacities returns the latest capacity snapshot for ALL active zones in one call.
	ListAllCapacities(ctx context.Context) ([]ZoneCapacityResponse, error)
	RecordCapacityEvent(ctx context.Context, zoneID, transactionID uuid.UUID, event types.ZoneEventType) error
}
