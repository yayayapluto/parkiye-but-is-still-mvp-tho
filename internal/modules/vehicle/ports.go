package vehicle

import (
	"context"

	"github.com/google/uuid"
)

type VehicleTypeRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*VehicleType, error)
	FindAll(ctx context.Context) ([]VehicleType, error)
	FindAllPaginated(ctx context.Context, search string, page, pageSize int) ([]VehicleType, int64, error)
	Create(ctx context.Context, vt *VehicleType) error
	Update(ctx context.Context, vt *VehicleType) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type VehicleRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Vehicle, error)
	FindByPlate(ctx context.Context, plate string) (*Vehicle, error)
	FindAll(ctx context.Context, filter ListVehicleFilter, page, pageSize int) ([]Vehicle, int64, error)
	Upsert(ctx context.Context, vehicle *Vehicle) (*Vehicle, error)
	Update(ctx context.Context, vehicle *Vehicle) error
}

type ServicePort interface {
	// Vehicle types
	ListVehicleTypes(ctx context.Context) ([]VehicleType, error)
	ListVehicleTypesPaginated(ctx context.Context, search string, page, pageSize int) ([]VehicleType, int64, error)
	GetVehicleType(ctx context.Context, id uuid.UUID) (*VehicleType, error)
	CreateVehicleType(ctx context.Context, req *CreateVehicleTypeRequest) (*VehicleType, error)
	UpdateVehicleType(ctx context.Context, id uuid.UUID, req *UpdateVehicleTypeRequest) (*VehicleType, error)
	DeleteVehicleType(ctx context.Context, id uuid.UUID) error

	// Vehicles
	ListVehicles(ctx context.Context, filter ListVehicleFilter, page, pageSize int) ([]Vehicle, int64, error)
	GetVehicle(ctx context.Context, id uuid.UUID) (*Vehicle, error)
	GetVehicleByPlate(ctx context.Context, plate string) (*Vehicle, error)
	UpsertVehicle(ctx context.Context, req *UpsertVehicleRequest) (*Vehicle, error)
	UpdateVehicle(ctx context.Context, id uuid.UUID, req *UpdateVehicleRequest) (*Vehicle, error)
}
