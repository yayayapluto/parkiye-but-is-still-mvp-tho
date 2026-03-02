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
	AdditionalFee    int        `gorm:"not null;default:0"` // flat surcharge on top of base fee
	ForVehicleTypeID *uuid.UUID `gorm:"type:uuid"`          // default vehicle type for OCR-created plates in this zone
	IsActive         bool       `gorm:"not null;default:true"`
	CreatedBy        *uuid.UUID `gorm:"type:uuid"`
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

func (Zone) TableName() string { return "zones" }

// ZoneCapacityLog is append-only; records each entry/exit event for capacity tracking.
// Current available = zones.capacity - latest occupied_count for that zone.
type ZoneCapacityLog struct {
	ID             uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ZoneID         uuid.UUID           `gorm:"type:uuid;not null;index"`
	TransactionID  uuid.UUID           `gorm:"type:uuid;not null;index"`
	EventType      types.ZoneEventType `gorm:"type:varchar(10);not null"` // "entry"|"exit"
	OccupiedCount  int                 `gorm:"not null"`
	AvailableCount int                 `gorm:"not null"`
	RecordedAt     time.Time           `gorm:"not null;default:now()"`

	Zone *Zone `gorm:"foreignKey:ZoneID"`
}

func (ZoneCapacityLog) TableName() string { return "zone_capacity_logs" }

// Gate is a physical entry or exit point in a zone.
type Gate struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ZoneID       uuid.UUID      `gorm:"type:uuid;not null;index"`
	Name         string         `gorm:"type:varchar(100);not null"`
	GateType     types.GateType `gorm:"type:varchar(10);not null"` // "entry"|"exit"
	LocationDesc string         `gorm:"type:text"`
	IsActive     bool           `gorm:"not null;default:true"`
	CreatedBy    *uuid.UUID     `gorm:"type:uuid"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"`

	Zone *Zone `gorm:"foreignKey:ZoneID"`
}

func (Gate) TableName() string { return "gates" }

// GateDevice is a support table for hardware-phase only (not active for simulation).
type GateDevice struct {
	ID           uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GateID       uuid.UUID          `gorm:"type:uuid;not null;index"`
	DeviceType   types.DeviceType   `gorm:"type:varchar(30);not null"` // "rfid_reader"|"printer"|"camera"|"barrier"|"qr_scanner"
	Status       types.DeviceStatus `gorm:"type:varchar(20);not null"` // "online"|"offline"|"error"
	LastPingAt   *time.Time
	ErrorMessage *string   `gorm:"type:text"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`

	Gate *Gate `gorm:"foreignKey:GateID"`
}

func (GateDevice) TableName() string { return "gate_devices" }
