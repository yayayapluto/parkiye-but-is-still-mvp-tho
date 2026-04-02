package zone

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

// Zone represents a named parking area with its own capacity and fee config.
type Zone struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name             string     `gorm:"type:varchar(100);not null"`
	Description      string     `gorm:"type:text"`
	Capacity         int        `gorm:"not null"`
	AdditionalFee    int        `gorm:"not null;default:0"`
	ForVehicleTypeID *uuid.UUID `gorm:"type:uuid"`
	IsActive         bool       `gorm:"not null;default:true"`
	CreatedBy        *uuid.UUID `gorm:"type:uuid"`
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

func (Zone) TableName() string { return "zones" }

// ZoneCapacityLog is append-only; records each entry/exit event for capacity tracking.
type ZoneCapacityLog struct {
	ID             uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ZoneID         uuid.UUID           `gorm:"type:uuid;not null;index"`
	TransactionID  uuid.UUID           `gorm:"type:uuid;not null;index"`
	EventType      types.ZoneEventType `gorm:"type:varchar(10);not null"`
	OccupiedCount  int                 `gorm:"not null"`
	AvailableCount int                 `gorm:"not null"`
	RecordedAt     time.Time           `gorm:"not null;default:now()"`

	Zone *Zone `gorm:"foreignKey:ZoneID"`
}

func (ZoneCapacityLog) TableName() string { return "zone_capacity_logs" }

// Gate is a physical entry or exit point in a zone.
type Gate struct {
	ID              uuid.UUID      `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	ZoneID          uuid.UUID      `gorm:"column:zone_id;type:uuid;not null;index"`
	Name            string         `gorm:"column:name;type:varchar(100);not null"`
	GateType        types.GateType `gorm:"column:gate_type;type:varchar(10);not null"`
	Mode            types.GateMode `gorm:"column:mode;type:varchar(20);not null;default:'manless'"`
	LocationDesc    string         `gorm:"column:location_desc;type:text"`
	GateToken       string         `gorm:"column:gate_token;type:varchar(60);uniqueIndex;not null"`
	TokenLastUsedAt *time.Time     `gorm:"column:token_last_used_at"`
	IsActive        bool           `gorm:"column:is_active;not null;default:true"`
	CreatedBy       *uuid.UUID     `gorm:"column:created_by;type:uuid"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime"`

	Zone *Zone `gorm:"foreignKey:ZoneID"`
}

func (Gate) TableName() string { return "gates" }

// GateCashierAssignment links a cashier user to a specific exit gate.
// Only one assignment per gate at any time (enforced via UNIQUE on gate_id).
// Only relevant for gates with mode = with_cashier.
type GateCashierAssignment struct {
	ID         uuid.UUID `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	GateID     uuid.UUID `gorm:"column:gate_id;type:uuid;not null;uniqueIndex"`
	UserID     uuid.UUID `gorm:"column:user_id;type:uuid;not null;index"`
	AssignedBy uuid.UUID `gorm:"column:assigned_by;type:uuid;not null"`
	AssignedAt time.Time `gorm:"column:assigned_at;not null;default:now()"`
}

func (GateCashierAssignment) TableName() string { return "gate_cashier_assignments" }

// GateDevice is a support table for hardware-phase only (not active for simulation).
type GateDevice struct {
	ID           uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GateID       uuid.UUID          `gorm:"type:uuid;not null;index"`
	DeviceType   types.DeviceType   `gorm:"type:varchar(30);not null"`
	Status       types.DeviceStatus `gorm:"type:varchar(20);not null"`
	LastPingAt   *time.Time
	ErrorMessage *string   `gorm:"type:text"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`

	Gate *Gate `gorm:"foreignKey:GateID"`
}

func (GateDevice) TableName() string { return "gate_devices" }
