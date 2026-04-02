package vehicle

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

type CreateVehicleTypeRequest struct {
	Name        string `json:"name"        validate:"required,min=2,max=50"`
	MinimumFee  int    `json:"minimum_fee" validate:"min=0"`
	Description string `json:"description"`
}

type UpdateVehicleTypeRequest struct {
	Name        *string `json:"name"        validate:"omitempty,min=2,max=50"`
	MinimumFee  *int    `json:"minimum_fee" validate:"omitempty,min=0"`
	Description *string `json:"description"`
}

type UpsertVehicleRequest struct {
	PlateNumber   string              `json:"plate_number"    validate:"required,min=2,max=20"`
	VehicleTypeID uuid.UUID           `json:"vehicle_type_id" validate:"required"`
	Source        types.VehicleSource `json:"source"          validate:"required,oneof=ocr manual_override"`
	Notes         string              `json:"notes"`
}

type UpdateVehicleRequest struct {
	VehicleTypeID *uuid.UUID `json:"vehicle_type_id" validate:"omitempty"`
	Notes         *string    `json:"notes"`
}

type VehicleTypeResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MinimumFee  int       `json:"minimum_fee"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type VehicleResponse struct {
	ID          uuid.UUID           `json:"id"`
	PlateNumber string              `json:"plate_number"`
	VehicleType VehicleTypeResponse `json:"vehicle_type"`
	Source      types.VehicleSource `json:"source"`
	Notes       string              `json:"notes"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

func toVehicleTypeResponse(vt *VehicleType) VehicleTypeResponse {
	return VehicleTypeResponse{
		ID:          vt.ID,
		Name:        vt.Name,
		MinimumFee:  vt.MinimumFee,
		Description: vt.Description,
		CreatedAt:   vt.CreatedAt,
	}
}

func toVehicleResponse(v *Vehicle) VehicleResponse {
	return VehicleResponse{
		ID:          v.ID,
		PlateNumber: v.PlateNumber,
		VehicleType: toVehicleTypeResponse(v.VehicleType),
		Source:      v.Source,
		Notes:       v.Notes,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

type ListVehicleFilter struct {
	PlateNumber   string
	VehicleTypeID *uuid.UUID
	Source        *types.VehicleSource
	Search        string
	DateFrom      *time.Time
	DateTo        *time.Time
	SortBy        string
	SortOrder     string
}
