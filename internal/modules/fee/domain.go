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
	ID                 uuid.UUID  `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	ZoneID             uuid.UUID  `gorm:"column:zone_id;type:uuid;not null;index"`
	VehicleTypeID      uuid.UUID  `gorm:"column:vehicle_type_id;type:uuid;not null;index"`
	BaseFee            int        `gorm:"column:base_fee;not null;default:0"`
	GracePeriodMinutes int        `gorm:"column:grace_period_minutes;not null;default:0"`
	IsActive           bool       `gorm:"column:is_active;not null;default:true"`
	EffectiveFrom      time.Time  `gorm:"column:effective_from;not null"`
	EffectiveUntil     *time.Time `gorm:"column:effective_until"`
	CreatedBy          *uuid.UUID `gorm:"column:created_by;type:uuid"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime"`

	Tiers []FeeTier `gorm:"foreignKey:FeeConfigID"`
}

func (FeeConfig) TableName() string { return "fee_configs" }

// FeeTier defines sequential billing tiers applied after the grace period.
// is_last_tier = true means that tier repeats for all remaining time blocks.
type FeeTier struct {
	ID              uuid.UUID `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	FeeConfigID     uuid.UUID `gorm:"column:fee_config_id;type:uuid;not null;index"`
	TierOrder       int       `gorm:"column:tier_order;not null"`
	DurationMinutes int       `gorm:"column:duration_minutes;not null"`
	FeeAmount       int       `gorm:"column:fee_amount;not null"`
	IsLastTier      bool      `gorm:"column:is_last_tier;not null;default:false"`

	FeeConfig *FeeConfig `gorm:"foreignKey:FeeConfigID"`
}

func (FeeTier) TableName() string { return "fee_tiers" }

// HolidayRate applies a multiplier or flat override fee for a date range.
// Overlap resolution: average all active multipliers.
type HolidayRate struct {
	ID                     uuid.UUID             `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Name                   string                `gorm:"column:name;type:varchar(100);not null"`
	DateStart              time.Time             `gorm:"column:date_start;type:date;not null"`
	DateEnd                time.Time             `gorm:"column:date_end;type:date;not null"`
	RateType               types.HolidayRateType `gorm:"column:rate_type;type:varchar(20);not null"`
	Multiplier             *decimal.Decimal      `gorm:"column:multiplier;type:decimal(5,2)"`
	OverrideFee            *int                  `gorm:"column:override_fee"`
	AppliesToZoneID        *uuid.UUID            `gorm:"column:applies_to_zone_id;type:uuid;index"`
	AppliesToVehicleTypeID *uuid.UUID            `gorm:"column:applies_to_vehicle_type_id;type:uuid;index"`
	CreatedBy              *uuid.UUID            `gorm:"column:created_by;type:uuid"`
	CreatedAt              time.Time             `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt              time.Time             `gorm:"column:updated_at;autoUpdateTime"`
}

func (HolidayRate) TableName() string { return "holiday_rates" }

// OCRConfig holds the auto-accept confidence threshold.
// Below threshold = queue for manual review.
type OCRConfig struct {
	ID                  uuid.UUID       `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	AutoAcceptThreshold decimal.Decimal `gorm:"column:auto_accept_threshold;type:decimal(5,4);not null"`
	IsActive            bool            `gorm:"column:is_active;not null;default:true"`
	EffectiveFrom       time.Time       `gorm:"column:effective_from;not null"`
	CreatedBy           *uuid.UUID      `gorm:"column:created_by;type:uuid"`
	CreatedAt           time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (OCRConfig) TableName() string { return "ocr_configs" }

// OverrideConfig configures max override count per operator before escalation.
type OverrideConfig struct {
	ID                     uuid.UUID  `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	MaxOverridesPerDay     int        `gorm:"column:max_overrides_per_day;not null"`
	MaxOverridesPerWeek    int        `gorm:"column:max_overrides_per_week;not null"`
	EscalationNotifyUserID *uuid.UUID `gorm:"column:escalation_notify_user_id;type:uuid"`
	IsActive               bool       `gorm:"column:is_active;not null;default:true"`
	CreatedBy              *uuid.UUID `gorm:"column:created_by;type:uuid"`
	CreatedAt              time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt              time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (OverrideConfig) TableName() string { return "override_configs" }
