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
	"parkieee/pkg/types"
)

type service struct {
	gateRepo       zoneDomain.GateRepositoryPort
	pairingRepo    PairingRepositoryPort
	assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort
	cfg            *config.Config
	log            logger.Logger
}

func NewService(
	gateRepo zoneDomain.GateRepositoryPort,
	pairingRepo PairingRepositoryPort,
	assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort,
	cfg *config.Config,
	log logger.Logger,
) ServicePort {
	return &service{
		gateRepo:       gateRepo,
		pairingRepo:    pairingRepo,
		assignmentRepo: assignmentRepo,
		cfg:            cfg,
		log:            log,
	}
}

func (s *service) Authenticate(ctx context.Context, gateToken string) (*GateAuthResponse, error) {
	gate, err := s.gateRepo.FindByToken(ctx, gateToken)
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "Token gate tidak valid")
	}
	if !gate.IsActive {
		return nil, errors.New(errors.ErrUnauthorized, "Gate tidak aktif")
	}

	token, expiresAt, err := s.generateGateJWT(gate)
	if err != nil {
		s.log.Error(ctx, "failed to generate gate JWT", "error", err, "gate_id", gate.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat token")
	}

	go func() {
		_ = s.gateRepo.UpdateTokenLastUsed(context.Background(), gate.ID)
	}()

	s.log.Info(ctx, "gate authenticated via token", "gate_id", gate.ID, "gate_name", gate.Name)

	return &GateAuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		Gate:      toGateInfo(gate),
	}, nil
}

func (s *service) ValidateGateToken(ctx context.Context, jwtToken string) (*middleware.GateClaims, error) {
	t, err := jwt.Parse(jwtToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			s.log.Warn(ctx, "validate gate token failed: unexpected signing method")
			return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
		}
		return []byte(s.cfg.GateJWTSecret()), nil
	})
	if err != nil || !t.Valid {
		s.log.Warn(ctx, "validate gate token failed: invalid or expired token")
		return nil, errors.New(errors.ErrUnauthorized, "Token gate tidak valid")
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		s.log.Warn(ctx, "validate gate token failed: cannot parse claims")
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
	}

	if kind, _ := claims["kind"].(string); kind != "gate" {
		s.log.Warn(ctx, "validate gate token failed: not a gate token", "kind", kind)
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
	}

	gateID, err := uuid.Parse(claims["gate_id"].(string))
	if err != nil {
		s.log.Warn(ctx, "validate gate token failed: invalid gate_id")
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
	}

	zoneID, err := uuid.Parse(claims["zone_id"].(string))
	if err != nil {
		s.log.Warn(ctx, "validate gate token failed: invalid zone_id")
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
	}

	s.log.Debug(ctx, "gate token validated", "gate_id", gateID, "zone_id", zoneID)
	return &middleware.GateClaims{
		GateID:   gateID,
		GateType: claims["gate_type"].(string),
		ZoneID:   zoneID,
		GateName: claims["gate_name"].(string),
	}, nil
}

func (s *service) RequestPairing(ctx context.Context, ip string) (*PairingResponse, error) {
	if err := s.pairingRepo.InvalidatePendingByIP(ctx, ip); err != nil {
		s.log.Error(ctx, "failed to invalidate old pairing codes", "ip", ip, "error", err)
	}

	code, err := generatePairingCode()
	if err != nil {
		s.log.Error(ctx, "failed to generate pairing code", "ip", ip, "error", err)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat kode pairing")
	}

	expiresAt := time.Now().Add(5 * time.Minute)

	qrContent, err := buildQRContent(s.cfg.BaseURL(), code, expiresAt)
	if err != nil {
		s.log.Error(ctx, "failed to build QR content", "code", code, "error", err)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat konten QR")
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
		s.log.Warn(ctx, "get pairing info failed: code not found", "code", code)
		return nil, errors.New(errors.ErrNotFound, "Kode pairing tidak ditemukan")
	}

	isExpired := time.Now().After(p.ExpiresAt)
	s.log.Debug(ctx, "pairing info fetched", "code", code, "status", p.Status, "is_expired", isExpired)

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

func (s *service) ConfirmPairing(ctx context.Context, code string, req ConfirmPairingRequest, adminID uuid.UUID) (*PairingConfirmResponse, error) {
	p, err := s.pairingRepo.FindByCode(ctx, code)
	if err != nil {
		s.log.Warn(ctx, "confirm pairing failed: code not found", "code", code, "admin_id", adminID)
		return nil, errors.New(errors.ErrNotFound, "Kode pairing tidak ditemukan")
	}
	if p.Status == "confirmed" {
		s.log.Warn(ctx, "confirm pairing failed: already confirmed", "code", code)
		return nil, errors.New(errors.ErrConflict, "Kode pairing sudah dikonfirmasi")
	}
	if time.Now().After(p.ExpiresAt) {
		s.log.Warn(ctx, "confirm pairing failed: code expired", "code", code)
		return nil, errors.New(errors.ErrValidation, "Kode pairing sudah kedaluwarsa")
	}

	gate, err := s.gateRepo.FindByID(ctx, req.GateID)
	if err != nil {
		s.log.Warn(ctx, "confirm pairing failed: gate not found", "gate_id", req.GateID)
		return nil, errors.New(errors.ErrNotFound, "Gate tidak ditemukan")
	}

	// Kalau gate exit dan cashier_user_id diisi → set mode with_cashier + buat assignment
	if gate.GateType == types.GateTypeExit && req.CashierUserID != nil {
		assignment := &zoneDomain.GateCashierAssignment{
			ID:         uuid.New(),
			GateID:     gate.ID,
			UserID:     *req.CashierUserID,
			AssignedBy: adminID,
			AssignedAt: time.Now(),
		}
		if err := s.assignmentRepo.Upsert(ctx, assignment); err != nil {
			s.log.Error(ctx, "confirm pairing: failed to upsert cashier assignment", "gate_id", gate.ID, "user_id", *req.CashierUserID, "error", err)
			return nil, err
		}
		if err := s.gateRepo.UpdateMode(ctx, gate.ID, types.GateModeWithCashier); err != nil {
			s.log.Error(ctx, "confirm pairing: failed to set gate mode with_cashier", "gate_id", gate.ID, "error", err)
			return nil, err
		}
		gate.Mode = types.GateModeWithCashier
		s.log.Info(ctx, "confirm pairing: cashier assigned and mode set to with_cashier", "gate_id", gate.ID, "cashier_user_id", *req.CashierUserID)
	}

	if !gate.IsActive {
		s.log.Warn(ctx, "confirm pairing failed: gate inactive", "gate_id", gate.ID)
		return nil, errors.New(errors.ErrValidation, "gate is inactive")
	}

	gateJWT, expiresAt, err := s.generateGateJWT(gate)
	if err != nil {
		s.log.Error(ctx, "failed to generate gate JWT for pairing", "error", err, "gate_id", gate.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat token gate")
	}

	if err := s.pairingRepo.Confirm(ctx, p.ID, req.GateID, adminID, gateJWT); err != nil {
		s.log.Error(ctx, "failed to confirm pairing", "code", code, "gate_id", req.GateID, "error", err)
		return nil, err
	}

	gate.IsActive = true
	if err := s.gateRepo.Update(ctx, gate); err != nil {
		s.log.Error(ctx, "failed to activate gate after pairing", "gate_id", gate.ID, "error", err)
	}

	if ch, ok := s.pairingRepo.GetSSEClient(code); ok {
		ch <- gateJWT
		s.pairingRepo.RemoveSSEClient(code)
	}

	go func() {
		_ = s.gateRepo.UpdateTokenLastUsed(context.Background(), gate.ID)
	}()

	s.log.Info(ctx, "pairing confirmed", "code", code, "gate_id", req.GateID, "admin_id", adminID)

	return &PairingConfirmResponse{
		GateJWT:   gateJWT,
		ExpiresAt: expiresAt,
		Gate:      toGateInfo(gate),
	}, nil
}

func (s *service) ListenPairing(ctx context.Context, code string) (<-chan string, error) {
	p, err := s.pairingRepo.FindByCode(ctx, code)
	if err != nil {
		s.log.Warn(ctx, "listen pairing failed: code not found", "code", code)
		return nil, errors.New(errors.ErrNotFound, "Kode pairing tidak ditemukan")
	}
	if p.Status == "confirmed" {
		s.log.Warn(ctx, "listen pairing failed: already confirmed", "code", code)
		return nil, errors.New(errors.ErrConflict, "Kode pairing sudah dikonfirmasi")
	}
	if time.Now().After(p.ExpiresAt) {
		s.log.Warn(ctx, "listen pairing failed: code expired", "code", code)
		return nil, errors.New(errors.ErrValidation, "Kode pairing sudah kedaluwarsa")
	}

	ch := make(chan string, 1)
	s.pairingRepo.SetSSEClient(code, ch)
	s.log.Info(ctx, "SSE pairing listener connected", "code", code)
	return ch, nil
}

func (s *service) generateGateJWT(gate *zoneDomain.Gate) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.cfg.JWT.GateTokenTTL)
	claims := jwt.MapClaims{
		"kind":      "gate",
		"gate_id":   gate.ID.String(),
		"gate_type": string(gate.GateType),
		"gate_mode": string(gate.Mode),
		"zone_id":   gate.ZoneID.String(),
		"gate_name": gate.Name,
		"exp":       expiresAt.Unix(),
		"iat":       time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(s.cfg.GateJWTSecret()))
	return token, expiresAt, err
}

func toGateInfo(gate *zoneDomain.Gate) GateInfo {
	zoneName := ""
	if gate.Zone != nil {
		zoneName = gate.Zone.Name
	}
	return GateInfo{
		ID:       gate.ID,
		Name:     gate.Name,
		GateType: gate.GateType,
		Mode:     gate.Mode,
		ZoneID:   gate.ZoneID,
		ZoneName: zoneName,
	}
}
