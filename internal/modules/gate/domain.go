package gate

import (
	"time"

	"github.com/google/uuid"
)

// GatePairingCode adalah record sementara yang dibuat screen gate
// saat pertama kali minta dipasangkan. Admin scan QR → confirm → screen dapat JWT.
type GatePairingCode struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	Code      string    `gorm:"column:code;type:varchar(6);uniqueIndex;not null"`          // 6 alphanumeric
	Status    string    `gorm:"column:status;type:varchar(20);not null;default:'pending'"` // "pending" | "confirmed"
	ExpiresAt time.Time `gorm:"column:expires_at;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`

	// Diisi saat admin confirm
	GateID      *uuid.UUID `gorm:"column:gate_id;type:uuid"`
	ConfirmedBy *uuid.UUID `gorm:"column:confirmed_by;type:uuid"` // user_id admin
	ConfirmedAt *time.Time `gorm:"column:confirmed_at"`
	GateJWT     *string    `gorm:"column:gate_jwt;type:text"` // JWT yang di-push ke screen via SSE

	// Audit
	IPAddress string `gorm:"column:ip_address;type:varchar(45);not null"` // IP screen yang request
}

func (GatePairingCode) TableName() string { return "gate_pairing_codes" }
