package vehicle

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"parkieee/pkg/errors"
)

type vehicleTypeRepo struct{ db *gorm.DB }
type vehicleRepo struct{ db *gorm.DB }

func NewVehicleTypeRepository(db *gorm.DB) VehicleTypeRepositoryPort {
	return &vehicleTypeRepo{db}
}

func NewVehicleRepository(db *gorm.DB) VehicleRepositoryPort {
	return &vehicleRepo{db}
}

func (r *vehicleTypeRepo) FindByID(ctx context.Context, id uuid.UUID) (*VehicleType, error) {
	var vt VehicleType
	if err := r.db.WithContext(ctx).First(&vt, "id = ?", id).Error; err != nil {
		return nil, errors.FromDB(err, "vehicle type not found")
	}
	return &vt, nil
}

func (r *vehicleTypeRepo) FindAll(ctx context.Context) ([]VehicleType, error) {
	var vts []VehicleType
	if err := r.db.WithContext(ctx).Order("name asc").Find(&vts).Error; err != nil {
		return nil, errors.FromDB(err, "vehicle types not found")
	}
	return vts, nil
}

func (r *vehicleTypeRepo) Create(ctx context.Context, vt *VehicleType) error {
	if err := r.db.WithContext(ctx).Create(vt).Error; err != nil {
		return errors.FromDB(err, "failed to create vehicle type")
	}
	return nil
}

func (r *vehicleTypeRepo) Update(ctx context.Context, vt *VehicleType) error {
	if err := r.db.WithContext(ctx).Save(vt).Error; err != nil {
		return errors.FromDB(err, "failed to update vehicle type")
	}
	return nil
}

func (r *vehicleTypeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&VehicleType{}, "id = ?", id)
	if res.Error != nil {
		return errors.FromDB(res.Error, "failed to delete vehicle type")
	}
	if res.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "vehicle type not found")
	}
	return nil
}

func (r *vehicleRepo) FindByID(ctx context.Context, id uuid.UUID) (*Vehicle, error) {
	var v Vehicle
	if err := r.db.WithContext(ctx).Preload("VehicleType").First(&v, "id = ?", id).Error; err != nil {
		return nil, errors.FromDB(err, "vehicle not found")
	}
	return &v, nil
}

func (r *vehicleRepo) FindByPlate(ctx context.Context, plate string) (*Vehicle, error) {
	var v Vehicle
	err := r.db.WithContext(ctx).Preload("VehicleType").
		Where("UPPER(plate_number) = UPPER(?)", strings.TrimSpace(plate)).
		First(&v).Error
	if err != nil {
		return nil, errors.FromDB(err, "vehicle not found")
	}
	return &v, nil
}

func (r *vehicleRepo) FindAll(ctx context.Context, typeID *uuid.UUID, page, pageSize int) ([]Vehicle, int64, error) {
	var vehicles []Vehicle
	var total int64

	q := r.db.WithContext(ctx).Model(&Vehicle{})
	if typeID != nil {
		q = q.Where("vehicle_type_id = ?", *typeID)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "failed to count vehicles")
	}

	offset := (page - 1) * pageSize
	if err := q.Preload("VehicleType").Order("created_at desc").Offset(offset).Limit(pageSize).Find(&vehicles).Error; err != nil {
		return nil, 0, errors.FromDB(err, "failed to list vehicles")
	}
	return vehicles, total, nil
}

func (r *vehicleRepo) Upsert(ctx context.Context, vehicle *Vehicle) (*Vehicle, error) {
	err := r.db.WithContext(ctx).
		Where(Vehicle{PlateNumber: strings.ToUpper(vehicle.PlateNumber)}).
		Assign(Vehicle{
			VehicleTypeID: vehicle.VehicleTypeID,
			Source:        vehicle.Source,
			Notes:         vehicle.Notes,
		}).
		Clauses(clause.OnConflict{UpdateAll: false}).
		FirstOrCreate(vehicle).Error
	if err != nil {
		return nil, errors.FromDB(err, "failed to upsert vehicle")
	}
	return r.FindByID(ctx, vehicle.ID)
}

func (r *vehicleRepo) Update(ctx context.Context, vehicle *Vehicle) error {
	if err := r.db.WithContext(ctx).Save(vehicle).Error; err != nil {
		return errors.FromDB(err, "failed to update vehicle")
	}
	return nil
}
