package gate

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/config"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/middleware"
)

type service struct {
	gateRepo    zoneDomain.GateRepositoryPort
	pairingRepo PairingRepositoryPort
	cfg         *config.Config
	log         logger.Logger
}

func NewService(
	gateRepo zoneDomain.GateRepositoryPort,
	pairingRepo PairingRepositoryPort,
	cfg *config.Config,
	log logger.Logger,
) ServicePort {
	return &service{
		gateRepo:    gateRepo,
		pairingRepo: pairingRepo,
		cfg:         cfg,
		log:         log,
	}
}

// ── Token-first flow ──────────────────────────────────────────────────────────

func (s *service) Authenticate(ctx context.Context, gateToken string) (*GateAuthResponse, error) {
	gate, err := s.gateRepo.FindByToken(ctx, gateToken)
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "invalid gate token")
	}

	if !gate.IsActive {
		return nil, errors.New(errors.ErrUnauthorized, "gate is inactive")
	}

	token, expiresAt, err := s.generateGateJWT(gate)
	if err != nil {
		s.log.Error(ctx, "failed to generate gate JWT", "error", err, "gate_id", gate.ID)
		return nil, errors.New(errors.ErrInternal, "failed to generate token")
	}

	go func() {
		_ = s.gateRepo.UpdateTokenLastUsed(context.Background(), gate.ID)
	}()

	s.log.Info(ctx, "gate authenticated via token", "gate_id", gate.ID, "gate_name", gate.Name)

	zoneName := ""
	if gate.Zone != nil {
		zoneName = gate.Zone.Name
	}

	return &GateAuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		Gate: GateInfo{
			ID:       gate.ID,
			Name:     gate.Name,
			GateType: gate.GateType,
			ZoneID:   gate.ZoneID,
			ZoneName: zoneName,
		},
	}, nil
}

func (s *service) ValidateGateToken(ctx context.Context, jwtToken string) (*middleware.GateClaims, error) {
	t, err := jwt.Parse(jwtToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(errors.ErrUnauthorized, "unexpected signing method")
		}
		return []byte(s.cfg.GateJWTSecret()), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New(errors.ErrUnauthorized, "invalid gate token")
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New(errors.ErrUnauthorized, "invalid token claims")
	}

	if kind, _ := claims["kind"].(string); kind != "gate" {
		return nil, errors.New(errors.ErrUnauthorized, "not a gate token")
	}

	gateID, err := uuid.Parse(claims["gate_id"].(string))
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "invalid gate_id in token")
	}

	zoneID, err := uuid.Parse(claims["zone_id"].(string))
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "invalid zone_id in token")
	}

	return &middleware.GateClaims{
		GateID:   gateID,
		GateType: claims["gate_type"].(string),
		ZoneID:   zoneID,
		GateName: claims["gate_name"].(string),
	}, nil
}

// ── QR Pairing flow ───────────────────────────────────────────────────────────

func (s *service) RequestPairing(ctx context.Context, ip string) (*PairingResponse, error) {
	// Expire paksa semua pending code dari IP yang sama — screen request baru
	if err := s.pairingRepo.InvalidatePendingByIP(ctx, ip); err != nil {
		s.log.Error(ctx, "failed to invalidate old pairing codes", "ip", ip, "error", err)
		// non-fatal, lanjut
	}

	code, err := generatePairingCode()
	if err != nil {
		s.log.Error(ctx, "failed to generate pairing code", "ip", ip, "error", err)
		return nil, errors.New(errors.ErrInternal, "failed to generate pairing code")
	}

	expiresAt := time.Now().Add(5 * time.Minute)

	qrContent, err := buildQRContent(s.cfg.BaseURL(), code, expiresAt)
	if err != nil {
		s.log.Error(ctx, "failed to build QR content", "code", code, "error", err)
		return nil, errors.New(errors.ErrInternal, "failed to build QR content")
	}

	pairing := &GatePairingCode{
		Code:      code,
		Status:    "pending",
		ExpiresAt: expiresAt,
		IPAddress: ip,
	}

	if err := s.pairingRepo.Create(ctx, pairing); err != nil {
		s.log.Error(ctx, "failed to create pairing code", "ip", ip, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "pairing code requested", "code", code, "ip", ip, "expires_at", expiresAt)

	return &PairingResponse{
		Code:      code,
		QRContent: qrContent,
		QRBase64:  encodeQRBase64(qrContent),
		ExpiresAt: expiresAt,
	}, nil
}

func (s *service) GetPairingInfo(ctx context.Context, code string) (*PairingInfoResponse, error) {
	p, err := s.pairingRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, errors.New(errors.ErrNotFound, "pairing code not found")
	}

	isExpired := time.Now().After(p.ExpiresAt)

	return &PairingInfoResponse{
		Code:        p.Code,
		Status:      p.Status,
		IsExpired:   isExpired,
		ExpiresAt:   p.ExpiresAt,
		CreatedAt:   p.CreatedAt,
		IPAddress:   p.IPAddress,
		GateID:      p.GateID,
		ConfirmedBy: p.ConfirmedBy,
		ConfirmedAt: p.ConfirmedAt,
	}, nil
}

func (s *service) ConfirmPairing(ctx context.Context, code string, gateID uuid.UUID, adminID uuid.UUID) (*PairingConfirmResponse, error) {
	p, err := s.pairingRepo.FindByCode(ctx, code)
	if err != nil {
		s.log.Warn(ctx, "confirm pairing failed: code not found", "code", code, "admin_id", adminID)
		return nil, errors.New(errors.ErrNotFound, "pairing code not found")
	}

	if p.Status == "confirmed" {
		s.log.Warn(ctx, "confirm pairing failed: already confirmed", "code", code)
		return nil, errors.New(errors.ErrConflict, "pairing code already confirmed")
	}

	if time.Now().After(p.ExpiresAt) {
		s.log.Warn(ctx, "confirm pairing failed: code expired", "code", code)
		return nil, errors.New(errors.ErrValidation, "pairing code has expired")
	}

	gate, err := s.gateRepo.FindByID(ctx, gateID)
	if err != nil {
		s.log.Warn(ctx, "confirm pairing failed: gate not found", "gate_id", gateID)
		return nil, errors.New(errors.ErrNotFound, "gate not found")
	}

	if !gate.IsActive {
		s.log.Warn(ctx, "confirm pairing failed: gate inactive", "gate_id", gateID)
		return nil, errors.New(errors.ErrValidation, "gate is inactive")
	}

	gateJWT, expiresAt, err := s.generateGateJWT(gate)
	if err != nil {
		s.log.Error(ctx, "failed to generate gate JWT for pairing", "error", err, "gate_id", gate.ID)
		return nil, errors.New(errors.ErrInternal, "failed to generate gate token")
	}

	if err := s.pairingRepo.Confirm(ctx, p.ID, gateID, adminID, gateJWT); err != nil {
		s.log.Error(ctx, "failed to confirm pairing", "code", code, "gate_id", gateID, "error", err)
		return nil, err
	}

	// Push JWT ke screen via SSE
	if ch, ok := s.pairingRepo.GetSSEClient(code); ok {
		ch <- gateJWT
		s.pairingRepo.RemoveSSEClient(code)
	}

	zoneName := ""
	if gate.Zone != nil {
		zoneName = gate.Zone.Name
	}

	s.log.Info(ctx, "pairing confirmed", "code", code, "gate_id", gateID, "admin_id", adminID)

	return &PairingConfirmResponse{
		GateJWT:   gateJWT,
		ExpiresAt: expiresAt,
		Gate: GateInfo{
			ID:       gate.ID,
			Name:     gate.Name,
			GateType: gate.GateType,
			ZoneID:   gate.ZoneID,
			ZoneName: zoneName,
		},
	}, nil
}

func (s *service) ListenPairing(ctx context.Context, code string) (<-chan string, error) {
	p, err := s.pairingRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, errors.New(errors.ErrNotFound, "pairing code not found")
	}

	if p.Status == "confirmed" {
		return nil, errors.New(errors.ErrConflict, "pairing code already confirmed")
	}

	if time.Now().After(p.ExpiresAt) {
		return nil, errors.New(errors.ErrValidation, "pairing code has expired")
	}

	// Buffer 1 supaya push tidak blocking
	ch := make(chan string, 1)
	s.pairingRepo.SetSSEClient(code, ch)

	return ch, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *service) generateGateJWT(gate *zoneDomain.Gate) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.cfg.JWT.GateTokenTTL)

	claims := jwt.MapClaims{
		"kind":      "gate",
		"gate_id":   gate.ID.String(),
		"gate_type": string(gate.GateType),
		"zone_id":   gate.ZoneID.String(),
		"gate_name": gate.Name,
		"exp":       expiresAt.Unix(),
		"iat":       time.Now().Unix(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(s.cfg.GateJWTSecret()))
	return token, expiresAt, err
}
