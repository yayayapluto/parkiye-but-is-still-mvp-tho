package vehicle

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

// VehicleType defines a category of vehicle with its minimum fee.
type VehicleType struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string    `gorm:"type:varchar(50);uniqueIndex;not null"` // "motorcycle"|"car"|"truck"
	MinimumFee  int       `gorm:"not null;default:0"`                    // minimum total fee regardless of duration
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (VehicleType) TableName() string { return "vehicle_types" }

// Vehicle is a unique plate number resolved via OCR or operator input.
type Vehicle struct {
	ID            uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PlateNumber   string              `gorm:"type:varchar(20);not null;index"`
	VehicleTypeID uuid.UUID           `gorm:"type:uuid;not null"`
	Source        types.VehicleSource `gorm:"type:varchar(20);not null"` // "ocr"|"manual_override"
	Notes         string              `gorm:"type:text"`
	CreatedAt     time.Time           `gorm:"autoCreateTime"`
	UpdatedAt     time.Time           `gorm:"autoUpdateTime"`

	VehicleType *VehicleType `gorm:"foreignKey:VehicleTypeID"`
}

func (Vehicle) TableName() string { return "vehicles" }
