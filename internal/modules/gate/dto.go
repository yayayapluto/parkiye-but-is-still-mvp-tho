package gate

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

type GateAuthRequest struct {
	GateToken string `json:"gate_token" validate:"required"`
}

type GateAuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Gate      GateInfo  `json:"gate"`
}

type GateInfo struct {
	ID       uuid.UUID      `json:"id"`
	Name     string         `json:"name"`
	GateType types.GateType `json:"gate_type"`
	Mode     types.GateMode `json:"mode"`
	ZoneID   uuid.UUID      `json:"zone_id"`
	ZoneName string         `json:"zone_name"`
}

type PairingResponse struct {
	Code      string    `json:"code"`
	QRContent string    `json:"qr_content"`
	QRBase64  string    `json:"qr_base64"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PairingInfoResponse struct {
	Code        string     `json:"code"`
	Status      string     `json:"status"`
	IsExpired   bool       `json:"is_expired"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	IPAddress   string     `json:"ip_address"`
	GateID      *uuid.UUID `json:"gate_id,omitempty"`
	ConfirmedBy *uuid.UUID `json:"confirmed_by,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// ConfirmPairingRequest adalah body request admin untuk confirm pairing.
// CashierUserID wajib diisi kalau gate bertipe exit dan mode with_cashier.
type ConfirmPairingRequest struct {
	GateID        uuid.UUID  `json:"gate_id"          validate:"required"`
	CashierUserID *uuid.UUID `json:"cashier_user_id"  validate:"omitempty"`
}

type PairingConfirmResponse struct {
	GateJWT   string    `json:"gate_jwt"`
	ExpiresAt time.Time `json:"expires_at"`
	Gate      GateInfo  `json:"gate"`
}
