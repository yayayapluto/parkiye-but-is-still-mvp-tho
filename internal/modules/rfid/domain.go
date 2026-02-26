package rfid

import (
	"time"

	"github.com/google/uuid"
)

// RFIDCard is self-registered on first tap. vehicle_id is null until OCR resolves and back-fills it.
type RFIDCard struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CardUID       string     `gorm:"type:varchar(64);uniqueIndex;not null"`
	VehicleID     *uuid.UUID `gorm:"type:uuid;index"`
	IsActive      bool       `gorm:"not null;default:true"`
	CreatedAt     time.Time  `gorm:"not null;default:now()"` // auto-set on first tap
	DeactivatedAt *time.Time
	DeactivatedBy *uuid.UUID `gorm:"type:uuid"` // operator who deactivated (e.g. lost card report)
}

func (RFIDCard) TableName() string { return "rfid_cards" }
