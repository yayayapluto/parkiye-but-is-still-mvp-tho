package gate

import (
	"context"

	"github.com/google/uuid"
	"parkieee/pkg/middleware"
)

// PairingRepositoryPort adalah kontrak repository untuk gate_pairing_codes.
type PairingRepositoryPort interface {
	Create(ctx context.Context, p *GatePairingCode) error
	FindByCode(ctx context.Context, code string) (*GatePairingCode, error)
	// InvalidatePendingByIP set expires_at = now() untuk semua pending code dari IP yang sama.
	// Dipanggil saat screen request pairing baru — code lama langsung expired paksa.
	InvalidatePendingByIP(ctx context.Context, ip string) error
	Confirm(ctx context.Context, id uuid.UUID, gateID uuid.UUID, confirmedBy uuid.UUID, gateJWT string) error
	// SetSSEClient menyimpan channel SSE untuk code tertentu (in-memory, bukan DB).
	// Hanya 1 listener per code — kalau ada yang baru, lama di-replace.
	SetSSEClient(code string, ch chan string)
	GetSSEClient(code string) (chan string, bool)
	RemoveSSEClient(code string)
}

// ServicePort adalah kontrak untuk gate service (autentikasi + pairing).
type ServicePort interface {
	// --- Token-first flow (existing) ---

	// Authenticate memvalidasi gate_token dan mengembalikan JWT gate session.
	Authenticate(ctx context.Context, gateToken string) (*GateAuthResponse, error)

	// ValidateGateToken memvalidasi JWT gate session per request.
	ValidateGateToken(ctx context.Context, jwtToken string) (*middleware.GateClaims, error)

	// --- QR Pairing flow (new) ---

	// RequestPairing dibuat oleh screen gate saat pertama kali setup.
	// Mengembalikan pairing code + QR content + expires_at.
	// Code lama dari IP yang sama di-expire paksa (invalidate).
	RequestPairing(ctx context.Context, ip string) (*PairingResponse, error)

	// GetPairingInfo dipanggil admin setelah scan QR.
	// Butuh auth token admin. Return info lengkap termasuk status expired.
	GetPairingInfo(ctx context.Context, code string) (*PairingInfoResponse, error)

	// ConfirmPairing dipanggil admin untuk assign screen ke gate tertentu.
	// Trigger push JWT ke screen via SSE.
	ConfirmPairing(ctx context.Context, code string, gateID uuid.UUID, adminID uuid.UUID) (*PairingConfirmResponse, error)

	// ListenPairing adalah SSE handler — screen subscribe dan nunggu konfirmasi.
	// Channel yang dikembalikan akan menerima JWT string saat admin confirm.
	// Koneksi lama di-kick kalau ada subscriber baru.
	ListenPairing(ctx context.Context, code string) (<-chan string, error)
}
