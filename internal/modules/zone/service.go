package zone

import (
	"context"

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
	return s.zoneRepo.FindByID(ctx, id)
}

func (s *service) ListZones(ctx context.Context, onlyActive bool, page, pageSize int) ([]Zone, int64, error) {
	return s.zoneRepo.FindAll(ctx, onlyActive, page, pageSize)
}

func (s *service) CreateZone(ctx context.Context, req *CreateZoneRequest, actorID uuid.UUID) (*Zone, error) {
	zone := &Zone{
		ID:            uuid.New(),
		Name:          req.Name,
		Description:   req.Description,
		Capacity:      req.Capacity,
		AdditionalFee: req.AdditionalFee,
		IsActive:      true,
		CreatedBy:     &actorID,
	}

	if err := s.zoneRepo.Create(ctx, zone); err != nil {
		return nil, err
	}

	s.log.Info(ctx, "zone created", "zone_id", zone.ID, "name", zone.Name, "actor_id", actorID)
	return s.zoneRepo.FindByID(ctx, zone.ID)
}

func (s *service) UpdateZone(ctx context.Context, id uuid.UUID, req *UpdateZoneRequest) (*Zone, error) {
	zone, err := s.zoneRepo.FindByID(ctx, id)
	if err != nil {
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

	if err := s.zoneRepo.Update(ctx, zone); err != nil {
		return nil, err
	}

	s.log.Info(ctx, "zone updated", "zone_id", id)
	return s.zoneRepo.FindByID(ctx, id)
}

func (s *service) DeactivateZone(ctx context.Context, id uuid.UUID) error {
	if err := s.zoneRepo.Deactivate(ctx, id); err != nil {
		return err
	}
	s.log.Info(ctx, "zone deactivated", "zone_id", id)
	return nil
}

func (s *service) GetGate(ctx context.Context, id uuid.UUID) (*Gate, error) {
	return s.gateRepo.FindByID(ctx, id)
}

func (s *service) ListGates(ctx context.Context, zoneID uuid.UUID, onlyActive bool) ([]Gate, error) {
	if _, err := s.zoneRepo.FindByID(ctx, zoneID); err != nil {
		return nil, errors.New(errors.ErrNotFound, "zone not found")
	}
	return s.gateRepo.FindByZoneID(ctx, zoneID, onlyActive)
}

func (s *service) CreateGate(ctx context.Context, req *CreateGateRequest, actorID uuid.UUID) (*Gate, error) {
	if _, err := s.zoneRepo.FindByID(ctx, req.ZoneID); err != nil {
		return nil, errors.New(errors.ErrNotFound, "zone not found")
	}

	gate := &Gate{
		ID:           uuid.New(),
		ZoneID:       req.ZoneID,
		Name:         req.Name,
		GateType:     req.GateType,
		LocationDesc: req.LocationDesc,
		IsActive:     true,
		CreatedBy:    &actorID,
	}

	if err := s.gateRepo.Create(ctx, gate); err != nil {
		return nil, err
	}

	s.log.Info(ctx, "gate created", "gate_id", gate.ID, "zone_id", gate.ZoneID, "actor_id", actorID)
	return s.gateRepo.FindByID(ctx, gate.ID)
}

func (s *service) UpdateGate(ctx context.Context, id uuid.UUID, req *UpdateGateRequest) (*Gate, error) {
	gate, err := s.gateRepo.FindByID(ctx, id)
	if err != nil {
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
		return nil, err
	}

	s.log.Info(ctx, "gate updated", "gate_id", id)
	return s.gateRepo.FindByID(ctx, id)
}

func (s *service) DeactivateGate(ctx context.Context, id uuid.UUID) error {
	if err := s.gateRepo.Deactivate(ctx, id); err != nil {
		return err
	}
	s.log.Info(ctx, "gate deactivated", "gate_id", id)
	return nil
}

func (s *service) GetCapacity(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityResponse, error) {
	zone, err := s.zoneRepo.FindByID(ctx, zoneID)
	if err != nil {
		return nil, err
	}

	occupied, available, err := GetCurrentOccupancy(ctx, s.db, zoneID, zone.Capacity)
	if err != nil {
		return nil, err
	}

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
		return err
	}

	latest, err := s.capacityRepo.LatestByZoneID(ctx, zoneID)
	if err != nil && !errors.IsCode(err, errors.ErrNotFound) {
		return err
	}

	occupied, available := NextOccupancy(latest, zone.Capacity, event)

	return s.capacityRepo.Append(ctx, &ZoneCapacityLog{
		ID:             uuid.New(),
		ZoneID:         zoneID,
		TransactionID:  transactionID,
		EventType:      event,
		OccupiedCount:  occupied,
		AvailableCount: available,
	})
}
