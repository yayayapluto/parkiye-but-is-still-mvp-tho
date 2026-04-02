package gate

import (
	"context"

	"github.com/google/uuid"
	"parkieee/pkg/middleware"
)

type PairingRepositoryPort interface {
	Create(ctx context.Context, p *GatePairingCode) error
	FindByCode(ctx context.Context, code string) (*GatePairingCode, error)
	InvalidatePendingByIP(ctx context.Context, ip string) error
	Confirm(ctx context.Context, id uuid.UUID, gateID uuid.UUID, confirmedBy uuid.UUID, gateJWT string) error
	SetSSEClient(code string, ch chan string)
	GetSSEClient(code string) (chan string, bool)
	RemoveSSEClient(code string)
}

type ServicePort interface {
	Authenticate(ctx context.Context, gateToken string) (*GateAuthResponse, error)
	ValidateGateToken(ctx context.Context, jwtToken string) (*middleware.GateClaims, error)

	RequestPairing(ctx context.Context, ip string) (*PairingResponse, error)
	GetPairingInfo(ctx context.Context, code string) (*PairingInfoResponse, error)
	// ConfirmPairing assign screen ke gate. Kalau gate exit + req.CashierUserID ada,
	// juga buat cashier assignment dan set mode gate ke with_cashier.
	ConfirmPairing(ctx context.Context, code string, req ConfirmPairingRequest, adminID uuid.UUID) (*PairingConfirmResponse, error)
	ListenPairing(ctx context.Context, code string) (<-chan string, error)
}
