package fee

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"parkieee/pkg/types"
)

// FeeConfig is versioned per zone + vehicle type.
// Conflict resolution: pick most recent effective_from.
type FeeConfig struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ZoneID             uuid.UUID  `gorm:"type:uuid;not null;index"`
	VehicleTypeID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	BaseFee            int        `gorm:"not null;default:0"` // flat fee applied at entry before tiers kick in
	GracePeriodMinutes int        `gorm:"not null;default:0"` // free window from entry before fee tiers start
	IsActive           bool       `gorm:"not null;default:true"`
	EffectiveFrom      time.Time  `gorm:"not null"`
	EffectiveUntil     *time.Time // null = open-ended, still active
	CreatedBy          *uuid.UUID `gorm:"type:uuid"`
	CreatedAt          time.Time  `gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime"`

	Tiers []FeeTier `gorm:"foreignKey:FeeConfigID"`
}

func (FeeConfig) TableName() string { return "fee_configs" }

// FeeTier defines sequential billing tiers applied after the grace period.
// is_last_tier = true means that tier repeats for all remaining time blocks.
type FeeTier struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	FeeConfigID     uuid.UUID `gorm:"type:uuid;not null;index"`
	TierOrder       int       `gorm:"not null"` // 1, 2, 3... applied in order
	DurationMinutes int       `gorm:"not null"`
	FeeAmount       int       `gorm:"not null"`
	IsLastTier      bool      `gorm:"not null;default:false"` // if true, repeats for all remaining blocks

	FeeConfig *FeeConfig `gorm:"foreignKey:FeeConfigID"`
}

func (FeeTier) TableName() string { return "fee_tiers" }

// HolidayRate applies a multiplier or flat override fee for a date range.
// Overlap resolution: average all active multipliers.
type HolidayRate struct {
	ID                     uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name                   string                `gorm:"type:varchar(100);not null"`
	DateStart              time.Time             `gorm:"type:date;not null"`
	DateEnd                time.Time             `gorm:"type:date;not null"`
	RateType               types.HolidayRateType `gorm:"type:varchar(20);not null"` // "multiplier"|"override"
	Multiplier             *decimal.Decimal      `gorm:"type:decimal(5,2)"`         // e.g. 1.5 = 150% of normal fee
	OverrideFee            *int                  // flat IDR amount, replaces entire calc
	AppliesToZoneID        *uuid.UUID            `gorm:"type:uuid;index"` // null = all zones
	AppliesToVehicleTypeID *uuid.UUID            `gorm:"type:uuid;index"` // null = all vehicle types
	CreatedBy              *uuid.UUID            `gorm:"type:uuid"`
	CreatedAt              time.Time             `gorm:"autoCreateTime"`
	UpdatedAt              time.Time             `gorm:"autoUpdateTime"`
}

func (HolidayRate) TableName() string { return "holiday_rates" }

// OCRConfig holds the auto-accept confidence threshold.
// Below threshold = queue for manual review.
type OCRConfig struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AutoAcceptThreshold decimal.Decimal `gorm:"type:decimal(5,4);not null"` // 0.0–1.0
	IsActive            bool            `gorm:"not null;default:true"`
	EffectiveFrom       time.Time       `gorm:"not null"`
	CreatedBy           *uuid.UUID      `gorm:"type:uuid"`
	CreatedAt           time.Time       `gorm:"autoCreateTime"`
}

func (OCRConfig) TableName() string { return "ocr_configs" }

// OverrideConfig configures max override count per operator before escalation.
type OverrideConfig struct {
	ID                     uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	MaxOverridesPerDay     int        `gorm:"not null"`
	MaxOverridesPerWeek    int        `gorm:"not null"`
	EscalationNotifyUserID *uuid.UUID `gorm:"type:uuid"` // user to notify when operator hits override limit
	IsActive               bool       `gorm:"not null;default:true"`
	CreatedBy              *uuid.UUID `gorm:"type:uuid"`
	CreatedAt              time.Time  `gorm:"autoCreateTime"`
	UpdatedAt              time.Time  `gorm:"autoUpdateTime"`
}

func (OverrideConfig) TableName() string { return "override_configs" }
