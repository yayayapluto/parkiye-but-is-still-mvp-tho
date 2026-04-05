package zone

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

type service struct {
	zoneRepo       ZoneRepositoryPort
	gateRepo       GateRepositoryPort
	assignmentRepo GateCashierAssignmentRepositoryPort
	capacityRepo   CapacityLogRepositoryPort
	db             *gorm.DB
	log            logger.Logger
}

func NewService(
	zoneRepo ZoneRepositoryPort,
	gateRepo GateRepositoryPort,
	assignmentRepo GateCashierAssignmentRepositoryPort,
	capacityRepo CapacityLogRepositoryPort,
	db *gorm.DB,
	log logger.Logger,
) ServicePort {
	return &service{
		zoneRepo:       zoneRepo,
		gateRepo:       gateRepo,
		assignmentRepo: assignmentRepo,
		capacityRepo:   capacityRepo,
		db:             db,
		log:            log,
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

func (s *service) ListZones(ctx context.Context, filter ListZoneFilter, page, pageSize int) ([]Zone, int64, error) {
	zones, total, err := s.zoneRepo.FindAll(ctx, filter, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list zones failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "zones listed", "count", len(zones), "total", total)
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

func (s *service) ListGates(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error) {
	if filter.ZoneID != nil {
		if _, err := s.zoneRepo.FindByID(ctx, *filter.ZoneID); err != nil {
			s.log.Warn(ctx, "list gates failed: zone not found", "zone_id", filter.ZoneID)
			return nil, 0, errors.New(errors.ErrNotFound, "Zona tidak ditemukan")
		}
	}
	gates, total, err := s.gateRepo.FindByZoneID(ctx, filter, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list gates failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "gates listed", "count", len(gates), "total", total)
	return gates, total, nil
}

func (s *service) ListAllGates(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error) {
	gates, total, err := s.gateRepo.FindAll(ctx, filter, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list all gates failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "all gates listed", "count", len(gates), "total", total)
	return gates, total, nil
}

func (s *service) CreateGate(ctx context.Context, req *CreateGateRequest, actorID uuid.UUID) (*Gate, error) {
	if _, err := s.zoneRepo.FindByID(ctx, req.ZoneID); err != nil {
		s.log.Warn(ctx, "create gate failed: zone not found", "zone_id", req.ZoneID)
		return nil, errors.New(errors.ErrNotFound, "Zona tidak ditemukan")
	}

	token, err := generateGateToken()
	if err != nil {
		s.log.Error(ctx, "failed to generate gate token", "error", err)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat token gate")
	}

	gate := &Gate{
		ID:           uuid.New(),
		ZoneID:       req.ZoneID,
		Name:         req.Name,
		GateType:     req.GateType,
		Mode:         types.GateModeManless,
		LocationDesc: req.LocationDesc,
		GateToken:    token,
		IsActive:     false,
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
		return nil, errors.New(errors.ErrInternal, "Gagal membuat token gate")
	}

	gate.GateToken = token
	if err := s.gateRepo.Update(ctx, gate); err != nil {
		s.log.Error(ctx, "failed to save regenerated gate token", "gate_id", id, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "gate token regenerated", "gate_id", id)
	return s.gateRepo.FindByID(ctx, id)
}

func (s *service) UpdateGateMode(ctx context.Context, id uuid.UUID, mode types.GateMode, actorID uuid.UUID) (*Gate, error) {
	gate, err := s.gateRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "update gate mode failed: gate not found", "gate_id", id)
		return nil, err
	}

	if gate.GateType != types.GateTypeExit {
		return nil, errors.New(errors.ErrValidation, "Mode gate hanya berlaku untuk gate keluar")
	}

	if mode == types.GateModeWithCashier {
		if _, err := s.assignmentRepo.FindByGateID(ctx, id); err != nil {
			s.log.Warn(ctx, "update gate mode failed: no cashier assigned", "gate_id", id)
			return nil, errors.New(errors.ErrValidation, "Tetapkan kasir ke gate ini sebelum beralih ke mode with_cashier")
		}
	}

	if err := s.gateRepo.UpdateMode(ctx, id, mode); err != nil {
		s.log.Error(ctx, "failed to update gate mode", "gate_id", id, "mode", mode, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "gate mode updated", "gate_id", id, "mode", mode, "actor_id", actorID)
	return s.gateRepo.FindByID(ctx, id)
}

func (s *service) AssignCashier(ctx context.Context, gateID, userID, assignedBy uuid.UUID) (*GateCashierAssignment, error) {
	gate, err := s.gateRepo.FindByID(ctx, gateID)
	if err != nil {
		s.log.Warn(ctx, "assign cashier failed: gate not found", "gate_id", gateID)
		return nil, err
	}

	if gate.GateType != types.GateTypeExit {
		return nil, errors.New(errors.ErrValidation, "Penugasan kasir hanya berlaku untuk gate keluar")
	}

	a := &GateCashierAssignment{
		ID:         uuid.New(),
		GateID:     gateID,
		UserID:     userID,
		AssignedBy: assignedBy,
		AssignedAt: time.Now(),
	}

	if err := s.assignmentRepo.Upsert(ctx, a); err != nil {
		s.log.Error(ctx, "failed to upsert cashier assignment", "gate_id", gateID, "user_id", userID, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "cashier assigned to gate", "gate_id", gateID, "user_id", userID, "assigned_by", assignedBy)
	return s.assignmentRepo.FindByGateID(ctx, gateID)
}

func (s *service) UnassignCashier(ctx context.Context, gateID uuid.UUID) error {
	gate, err := s.gateRepo.FindByID(ctx, gateID)
	if err != nil {
		s.log.Warn(ctx, "unassign cashier failed: gate not found", "gate_id", gateID)
		return err
	}

	if gate.Mode == types.GateModeWithCashier {
		return errors.New(errors.ErrValidation, "Ubah mode gate ke manless sebelum menghapus penugasan kasir")
	}

	if err := s.assignmentRepo.DeleteByGateID(ctx, gateID); err != nil {
		s.log.Error(ctx, "failed to delete cashier assignment", "gate_id", gateID, "error", err)
		return err
	}

	s.log.Info(ctx, "cashier unassigned from gate", "gate_id", gateID)
	return nil
}

func (s *service) GetCashierAssignment(ctx context.Context, gateID uuid.UUID) (*GateCashierAssignment, error) {
	a, err := s.assignmentRepo.FindByGateID(ctx, gateID)
	if err != nil {
		s.log.Debug(ctx, "get cashier assignment: none", "gate_id", gateID)
		return nil, err
	}
	return a, nil
}

func (s *service) GetAssignmentByUser(ctx context.Context, userID uuid.UUID) (*GateCashierAssignment, error) {
	a, err := s.assignmentRepo.FindByUserID(ctx, userID)
	if err != nil {
		s.log.Debug(ctx, "get assignment by user: none", "user_id", userID)
		return nil, err
	}
	return a, nil
}

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

// ListAllCapacities returns the latest capacity snapshot for ALL active zones
// in a single aggregated query via the repository.
func (s *service) ListAllCapacities(ctx context.Context) ([]ZoneCapacityResponse, error) {
	result, err := s.capacityRepo.AllCapacities(ctx)
	if err != nil {
		s.log.Error(ctx, "list all capacities failed", "error", err)
		return nil, err
	}
	s.log.Debug(ctx, "all zone capacities fetched", "count", len(result))
	return result, nil
}

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
