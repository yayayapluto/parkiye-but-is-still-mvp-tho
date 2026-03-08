package zone

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

type service struct {
	zoneRepo     ZoneRepositoryPort
	gateRepo     GateRepositoryPort
	capacityRepo CapacityLogRepositoryPort
	db           *gorm.DB
	log          logger.Logger
}

func NewService(
	zoneRepo ZoneRepositoryPort,
	gateRepo GateRepositoryPort,
	capacityRepo CapacityLogRepositoryPort,
	db *gorm.DB,
	log logger.Logger,
) ServicePort {
	return &service{
		zoneRepo:     zoneRepo,
		gateRepo:     gateRepo,
		capacityRepo: capacityRepo,
		db:           db,
		log:          log,
	}
}

func (s *service) GetZone(ctx context.Context, id uuid.UUID) (*Zone, error) {
	zone, err := s.zoneRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get zone failed: not found", "zone_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "zone fetched", "zone_id", id)
	return zone, nil
}

func (s *service) ListZones(ctx context.Context, onlyActive bool, page, pageSize int) ([]Zone, int64, error) {
	zones, total, err := s.zoneRepo.FindAll(ctx, onlyActive, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list zones failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "zones listed", "count", len(zones), "total", total, "only_active", onlyActive)
	return zones, total, nil
}

func (s *service) CreateZone(ctx context.Context, req *CreateZoneRequest, actorID uuid.UUID) (*Zone, error) {
	zone := &Zone{
		ID:               uuid.New(),
		Name:             req.Name,
		Description:      req.Description,
		Capacity:         req.Capacity,
		AdditionalFee:    req.AdditionalFee,
		ForVehicleTypeID: req.ForVehicleTypeID,
		IsActive:         true,
		CreatedBy:        &actorID,
	}

	if err := s.zoneRepo.Create(ctx, zone); err != nil {
		s.log.Error(ctx, "failed to create zone", "name", zone.Name, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "zone created", "zone_id", zone.ID, "name", zone.Name, "actor_id", actorID)
	return s.zoneRepo.FindByID(ctx, zone.ID)
}

func (s *service) UpdateZone(ctx context.Context, id uuid.UUID, req *UpdateZoneRequest) (*Zone, error) {
	zone, err := s.zoneRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "update zone failed: not found", "zone_id", id)
		return nil, err
	}

	if req.Name != nil {
		zone.Name = *req.Name
	}
	if req.Description != nil {
		zone.Description = *req.Description
	}
	if req.Capacity != nil {
		zone.Capacity = *req.Capacity
	}
	if req.AdditionalFee != nil {
		zone.AdditionalFee = *req.AdditionalFee
	}
	if req.IsActive != nil {
		zone.IsActive = *req.IsActive
	}
	if req.ForVehicleTypeID != nil {
		zone.ForVehicleTypeID = req.ForVehicleTypeID
	}

	if err := s.zoneRepo.Update(ctx, zone); err != nil {
		s.log.Error(ctx, "failed to update zone", "zone_id", id, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "zone updated", "zone_id", id)
	return s.zoneRepo.FindByID(ctx, id)
}

func (s *service) DeactivateZone(ctx context.Context, id uuid.UUID) error {
	if err := s.zoneRepo.Deactivate(ctx, id); err != nil {
		s.log.Error(ctx, "failed to deactivate zone", "zone_id", id, "error", err)
		return err
	}
	s.log.Info(ctx, "zone deactivated", "zone_id", id)
	return nil
}

func (s *service) GetGate(ctx context.Context, id uuid.UUID) (*Gate, error) {
	gate, err := s.gateRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get gate failed: not found", "gate_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "gate fetched", "gate_id", id)
	return gate, nil
}

func (s *service) ListGates(ctx context.Context, zoneID uuid.UUID, onlyActive bool, page, pageSize int) ([]Gate, int64, error) {
	if _, err := s.zoneRepo.FindByID(ctx, zoneID); err != nil {
		s.log.Warn(ctx, "list gates failed: zone not found", "zone_id", zoneID)
		return nil, 0, errors.New(errors.ErrNotFound, "zone not found")
	}
	gates, total, err := s.gateRepo.FindByZoneID(ctx, zoneID, onlyActive, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list gates failed", "zone_id", zoneID, "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "gates listed", "zone_id", zoneID, "count", len(gates), "total", total)
	return gates, total, nil
}

func (s *service) CreateGate(ctx context.Context, req *CreateGateRequest, actorID uuid.UUID) (*Gate, error) {
	if _, err := s.zoneRepo.FindByID(ctx, req.ZoneID); err != nil {
		s.log.Warn(ctx, "create gate failed: zone not found", "zone_id", req.ZoneID)
		return nil, errors.New(errors.ErrNotFound, "zone not found")
	}

	token, err := generateGateToken()
	if err != nil {
		s.log.Error(ctx, "failed to generate gate token", "error", err)
		return nil, errors.New(errors.ErrInternal, "failed to generate gate token")
	}

	gate := &Gate{
		ID:           uuid.New(),
		ZoneID:       req.ZoneID,
		Name:         req.Name,
		GateType:     req.GateType,
		LocationDesc: req.LocationDesc,
		GateToken:    token,
		IsActive:     true,
		CreatedBy:    &actorID,
	}

	if err := s.gateRepo.Create(ctx, gate); err != nil {
		s.log.Error(ctx, "failed to create gate", "zone_id", gate.ZoneID, "name", gate.Name, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "gate created", "gate_id", gate.ID, "zone_id", gate.ZoneID, "actor_id", actorID)
	return s.gateRepo.FindByID(ctx, gate.ID)
}

func (s *service) UpdateGate(ctx context.Context, id uuid.UUID, req *UpdateGateRequest) (*Gate, error) {
	gate, err := s.gateRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "update gate failed: not found", "gate_id", id)
		return nil, err
	}

	if req.Name != nil {
		gate.Name = *req.Name
	}
	if req.GateType != nil {
		gate.GateType = *req.GateType
	}
	if req.LocationDesc != nil {
		gate.LocationDesc = *req.LocationDesc
	}
	if req.IsActive != nil {
		gate.IsActive = *req.IsActive
	}

	if err := s.gateRepo.Update(ctx, gate); err != nil {
		s.log.Error(ctx, "failed to update gate", "gate_id", id, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "gate updated", "gate_id", id)
	return s.gateRepo.FindByID(ctx, id)
}

func (s *service) DeactivateGate(ctx context.Context, id uuid.UUID) error {
	if err := s.gateRepo.Deactivate(ctx, id); err != nil {
		s.log.Error(ctx, "failed to deactivate gate", "gate_id", id, "error", err)
		return err
	}
	s.log.Info(ctx, "gate deactivated", "gate_id", id)
	return nil
}

func (s *service) RegenerateGateToken(ctx context.Context, id uuid.UUID) (*Gate, error) {
	gate, err := s.gateRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "regenerate gate token failed: gate not found", "gate_id", id)
		return nil, err
	}

	token, err := generateGateToken()
	if err != nil {
		s.log.Error(ctx, "failed to generate new gate token", "gate_id", id, "error", err)
		return nil, errors.New(errors.ErrInternal, "failed to generate gate token")
	}

	gate.GateToken = token
	if err := s.gateRepo.Update(ctx, gate); err != nil {
		s.log.Error(ctx, "failed to save regenerated gate token", "gate_id", id, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "gate token regenerated", "gate_id", id)
	return s.gateRepo.FindByID(ctx, id)
}

// generateGateToken menghasilkan token unik format: gat_ + 32 hex chars (16 random bytes).
func generateGateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "gat_" + hex.EncodeToString(b), nil
}

func (s *service) GetCapacity(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityResponse, error) {
	zone, err := s.zoneRepo.FindByID(ctx, zoneID)
	if err != nil {
		s.log.Warn(ctx, "get capacity failed: zone not found", "zone_id", zoneID)
		return nil, err
	}

	occupied, available, err := GetCurrentOccupancy(ctx, s.db, zoneID, zone.Capacity)
	if err != nil {
		s.log.Error(ctx, "get capacity failed: cannot read occupancy", "zone_id", zoneID, "error", err)
		return nil, err
	}

	s.log.Debug(ctx, "zone capacity fetched", "zone_id", zoneID, "occupied", occupied, "available", available, "capacity", zone.Capacity)
	return &ZoneCapacityResponse{
		ZoneID:         zone.ID,
		ZoneName:       zone.Name,
		Capacity:       zone.Capacity,
		OccupiedCount:  occupied,
		AvailableCount: available,
	}, nil
}

// RecordCapacityEvent is called by the transaction module after a confirmed entry or exit.
// It reads the latest log, computes new counts, then appends a new row.
func (s *service) RecordCapacityEvent(ctx context.Context, zoneID, transactionID uuid.UUID, event types.ZoneEventType) error {
	zone, err := s.zoneRepo.FindByID(ctx, zoneID)
	if err != nil {
		s.log.Error(ctx, "record capacity event failed: zone not found", "zone_id", zoneID, "error", err)
		return err
	}

	latest, err := s.capacityRepo.LatestByZoneID(ctx, zoneID)
	if err != nil && !errors.IsCode(err, errors.ErrNotFound) {
		s.log.Error(ctx, "record capacity event failed: cannot fetch latest log", "zone_id", zoneID, "error", err)
		return err
	}

	occupied, available := NextOccupancy(latest, zone.Capacity, event)

	if err := s.capacityRepo.Append(ctx, &ZoneCapacityLog{
		ID:             uuid.New(),
		ZoneID:         zoneID,
		TransactionID:  transactionID,
		EventType:      event,
		OccupiedCount:  occupied,
		AvailableCount: available,
	}); err != nil {
		s.log.Error(ctx, "record capacity event failed: append log", "zone_id", zoneID, "transaction_id", transactionID, "event", event, "error", err)
		return err
	}
	s.log.Info(ctx, "capacity event recorded", "zone_id", zoneID, "transaction_id", transactionID, "event", event, "occupied", occupied, "available", available)
	return nil
}
