package zone

import (
	"context"

	"github.com/google/uuid"
)

type ZoneRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Zone, error)
	FindAll(ctx context.Context, onlyActive bool, page, pageSize int) ([]Zone, int64, error)
	Create(ctx context.Context, zone *Zone) error
	Update(ctx context.Context, zone *Zone) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type GateRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Gate, error)
	FindByZoneID(ctx context.Context, zoneID uuid.UUID, onlyActive bool) ([]Gate, error)
	Create(ctx context.Context, gate *Gate) error
	Update(ctx context.Context, gate *Gate) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type GateDeviceRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*GateDevice, error)
	FindByGateID(ctx context.Context, gateID uuid.UUID) ([]GateDevice, error)
	Upsert(ctx context.Context, device *GateDevice) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CapacityLogRepositoryPort interface {
	// Append records an entry or exit event. OccupiedCount and AvailableCount
	// must be pre-computed by the caller before appending.
	Append(ctx context.Context, log *ZoneCapacityLog) error

	// LatestByZoneID returns the most recent log row for a zone,
	// used to get current occupied/available counts without a full COUNT query.
	LatestByZoneID(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityLog, error)
}

type ServicePort interface {
	// Zone management
	GetZone(ctx context.Context, id uuid.UUID) (*Zone, error)
	ListZones(ctx context.Context, onlyActive bool, page, pageSize int) ([]Zone, int64, error)
	CreateZone(ctx context.Context, req *CreateZoneRequest, actorID uuid.UUID) (*Zone, error)
	UpdateZone(ctx context.Context, id uuid.UUID, req *UpdateZoneRequest) (*Zone, error)
	DeactivateZone(ctx context.Context, id uuid.UUID) error

	// Gate management
	GetGate(ctx context.Context, id uuid.UUID) (*Gate, error)
	ListGates(ctx context.Context, zoneID uuid.UUID, onlyActive bool) ([]Gate, error)
	CreateGate(ctx context.Context, req *CreateGateRequest, actorID uuid.UUID) (*Gate, error)
	UpdateGate(ctx context.Context, id uuid.UUID, req *UpdateGateRequest) (*Gate, error)
	DeactivateGate(ctx context.Context, id uuid.UUID) error

	// Capacity
	GetCapacity(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityResponse, error)
}
