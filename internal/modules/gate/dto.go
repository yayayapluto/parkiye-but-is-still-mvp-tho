package gate

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

// ── Token-first flow ──────────────────────────────────────────────────────────

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
	ZoneID   uuid.UUID      `json:"zone_id"`
	ZoneName string         `json:"zone_name"`
}

// ── QR Pairing flow ───────────────────────────────────────────────────────────

// PairingResponse dikembalikan ke screen setelah request pairing code.
type PairingResponse struct {
	Code      string    `json:"code"`       // 6 alphanumeric, e.g. "A3K9XZ"
	QRContent string    `json:"qr_content"` // JSON string yang di-encode ke QR
	QRBase64  string    `json:"qr_base64"`  // base64 dari qr_content — frontend pakai ini untuk generate QR
	ExpiresAt time.Time `json:"expires_at"`
}

// PairingInfoResponse dikembalikan ke admin setelah scan QR.
type PairingInfoResponse struct {
	Code        string     `json:"code"`
	Status      string     `json:"status"`     // "pending" | "confirmed"
	IsExpired   bool       `json:"is_expired"` // true kalau expires_at < now
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	IPAddress   string     `json:"ip_address"`
	GateID      *uuid.UUID `json:"gate_id,omitempty"`
	ConfirmedBy *uuid.UUID `json:"confirmed_by,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// ConfirmPairingRequest adalah body request admin untuk confirm pairing.
type ConfirmPairingRequest struct {
	GateID uuid.UUID `json:"gate_id" validate:"required"`
}

// PairingConfirmResponse dikembalikan ke admin setelah confirm.
// JWT yang sama juga di-push ke screen via SSE.
type PairingConfirmResponse struct {
	GateJWT   string    `json:"gate_jwt"`
	ExpiresAt time.Time `json:"expires_at"`
	Gate      GateInfo  `json:"gate"`
}
