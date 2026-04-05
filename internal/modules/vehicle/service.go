package vehicle

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
)

type service struct {
	vtRepo VehicleTypeRepositoryPort
	vRepo  VehicleRepositoryPort
	log    logger.Logger
}

func NewService(vtRepo VehicleTypeRepositoryPort, vRepo VehicleRepositoryPort, log logger.Logger) ServicePort {
	return &service{vtRepo: vtRepo, vRepo: vRepo, log: log}
}

func (s *service) ListVehicleTypes(ctx context.Context) ([]VehicleType, error) {
	types, err := s.vtRepo.FindAll(ctx)
	if err != nil {
		s.log.Error(ctx, "list vehicle types failed", "error", err)
		return nil, err
	}
	s.log.Debug(ctx, "vehicle types listed", "count", len(types))
	return types, nil
}

func (s *service) ListVehicleTypesPaginated(ctx context.Context, search string, page, pageSize int) ([]VehicleType, int64, error) {
	vts, total, err := s.vtRepo.FindAllPaginated(ctx, search, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list vehicle types paginated failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "vehicle types listed (paginated)", "count", len(vts), "total", total)
	return vts, total, nil
}

func (s *service) GetVehicleType(ctx context.Context, id uuid.UUID) (*VehicleType, error) {
	vt, err := s.vtRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get vehicle type failed: not found", "vehicle_type_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "vehicle type fetched", "vehicle_type_id", id)
	return vt, nil
}

func (s *service) CreateVehicleType(ctx context.Context, req *CreateVehicleTypeRequest) (*VehicleType, error) {
	vt := &VehicleType{
		ID:          uuid.New(),
		Name:        strings.ToLower(strings.TrimSpace(req.Name)),
		MinimumFee:  req.MinimumFee,
		Description: req.Description,
	}
	if err := s.vtRepo.Create(ctx, vt); err != nil {
		s.log.Error(ctx, "failed to create vehicle type", "name", vt.Name, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "vehicle type created", "id", vt.ID, "name", vt.Name)
	return s.vtRepo.FindByID(ctx, vt.ID)
}

func (s *service) UpdateVehicleType(ctx context.Context, id uuid.UUID, req *UpdateVehicleTypeRequest) (*VehicleType, error) {
	vt, err := s.vtRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "update vehicle type failed: not found", "id", id)
		return nil, err
	}
	if req.Name != nil {
		vt.Name = strings.ToLower(strings.TrimSpace(*req.Name))
	}
	if req.MinimumFee != nil {
		vt.MinimumFee = *req.MinimumFee
	}
	if req.Description != nil {
		vt.Description = *req.Description
	}
	if err := s.vtRepo.Update(ctx, vt); err != nil {
		s.log.Error(ctx, "failed to update vehicle type", "id", id, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "vehicle type updated", "id", id)
	return s.vtRepo.FindByID(ctx, id)
}

func (s *service) DeleteVehicleType(ctx context.Context, id uuid.UUID) error {
	if err := s.vtRepo.Delete(ctx, id); err != nil {
		s.log.Error(ctx, "failed to delete vehicle type", "id", id, "error", err)
		return err
	}
	s.log.Info(ctx, "vehicle type deleted", "id", id)
	return nil
}

func (s *service) ListVehicles(ctx context.Context, filter ListVehicleFilter, page, pageSize int) ([]Vehicle, int64, error) {
	vehicles, total, err := s.vRepo.FindAll(ctx, filter, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list vehicles failed", "filter", filter, "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "vehicles listed", "count", len(vehicles), "total", total, "filter", filter)
	return vehicles, total, nil
}

func (s *service) GetVehicle(ctx context.Context, id uuid.UUID) (*Vehicle, error) {
	v, err := s.vRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get vehicle failed: not found", "vehicle_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "vehicle fetched", "vehicle_id", id, "plate", v.PlateNumber)
	return v, nil
}

func (s *service) GetVehicleByPlate(ctx context.Context, plate string) (*Vehicle, error) {
	v, err := s.vRepo.FindByPlate(ctx, plate)
	if err != nil {
		s.log.Warn(ctx, "get vehicle by plate failed: not found", "plate", plate)
		return nil, err
	}
	s.log.Debug(ctx, "vehicle fetched by plate", "vehicle_id", v.ID, "plate", plate)
	return v, nil
}

func (s *service) UpsertVehicle(ctx context.Context, req *UpsertVehicleRequest) (*Vehicle, error) {
	if _, err := s.vtRepo.FindByID(ctx, req.VehicleTypeID); err != nil {
		s.log.Warn(ctx, "upsert vehicle failed: vehicle type not found", "vehicle_type_id", req.VehicleTypeID)
		return nil, errors.New(errors.ErrNotFound, "Jenis kendaraan tidak ditemukan")
	}

	v := &Vehicle{
		ID:            uuid.New(),
		PlateNumber:   strings.ToUpper(strings.TrimSpace(req.PlateNumber)),
		VehicleTypeID: req.VehicleTypeID,
		Source:        req.Source,
		Notes:         req.Notes,
	}

	result, err := s.vRepo.Upsert(ctx, v)
	if err != nil {
		s.log.Error(ctx, "failed to upsert vehicle", "plate", v.PlateNumber, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "vehicle upserted", "id", result.ID, "plate", result.PlateNumber)
	return result, nil
}

func (s *service) UpdateVehicle(ctx context.Context, id uuid.UUID, req *UpdateVehicleRequest) (*Vehicle, error) {
	v, err := s.vRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "update vehicle failed: not found", "id", id)
		return nil, err
	}
	if req.VehicleTypeID != nil {
		if _, err := s.vtRepo.FindByID(ctx, *req.VehicleTypeID); err != nil {
			s.log.Warn(ctx, "update vehicle failed: vehicle type not found", "vehicle_type_id", req.VehicleTypeID)
			return nil, errors.New(errors.ErrNotFound, "Jenis kendaraan tidak ditemukan")
		}
		v.VehicleTypeID = *req.VehicleTypeID
	}
	if req.Notes != nil {
		v.Notes = *req.Notes
	}
	if err := s.vRepo.Update(ctx, v); err != nil {
		s.log.Error(ctx, "failed to update vehicle", "id", id, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "vehicle updated", "id", id)
	return s.vRepo.FindByID(ctx, id)
}
