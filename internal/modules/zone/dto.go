package zone

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

type CreateZoneRequest struct {
	Name             string     `json:"name"                validate:"required,min=2,max=100"`
	Description      string     `json:"description"         validate:"omitempty,max=500"`
	Capacity         int        `json:"capacity"            validate:"required,min=1"`
	AdditionalFee    int        `json:"additional_fee"      validate:"min=0"`
	ForVehicleTypeID *uuid.UUID `json:"for_vehicle_type_id" validate:"omitempty,uuid4"` // default vehicle type for OCR in this zone
}

type UpdateZoneRequest struct {
	Name             *string    `json:"name"                validate:"omitempty,min=2,max=100"`
	Description      *string    `json:"description"         validate:"omitempty,max=500"`
	Capacity         *int       `json:"capacity"            validate:"omitempty,min=1"`
	AdditionalFee    *int       `json:"additional_fee"      validate:"omitempty,min=0"`
	ForVehicleTypeID *uuid.UUID `json:"for_vehicle_type_id" validate:"omitempty,uuid4"`
	IsActive         *bool      `json:"is_active"`
}

type CreateGateRequest struct {
	ZoneID       uuid.UUID      `json:"zone_id"       validate:"required"`
	Name         string         `json:"name"          validate:"required,min=2,max=100"`
	GateType     types.GateType `json:"gate_type"     validate:"required,oneof=entry exit"`
	LocationDesc string         `json:"location_desc" validate:"omitempty,max=500"`
}

type UpdateGateRequest struct {
	Name         *string         `json:"name"          validate:"omitempty,min=2,max=100"`
	GateType     *types.GateType `json:"gate_type"     validate:"omitempty,oneof=entry exit"`
	LocationDesc *string         `json:"location_desc" validate:"omitempty,max=500"`
	IsActive     *bool           `json:"is_active"`
}

type ZoneResponse struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	Capacity         int        `json:"capacity"`
	AdditionalFee    int        `json:"additional_fee"`
	ForVehicleTypeID *uuid.UUID `json:"for_vehicle_type_id"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type GateResponse struct {
	ID              uuid.UUID      `json:"id"`
	ZoneID          uuid.UUID      `json:"zone_id"`
	Name            string         `json:"name"`
	GateType        types.GateType `json:"gate_type"`
	LocationDesc    string         `json:"location_desc"`
	GateToken       string         `json:"gate_token"` // hanya tampil ke admin — jangan expose ke publik
	TokenLastUsedAt *time.Time     `json:"token_last_used_at"`
	IsActive        bool           `json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type ZoneCapacityResponse struct {
	ZoneID         uuid.UUID `json:"zone_id"`
	ZoneName       string    `json:"zone_name"`
	Capacity       int       `json:"capacity"`
	OccupiedCount  int       `json:"occupied_count"`
	AvailableCount int       `json:"available_count"`
}

func toZoneResponse(z *Zone) ZoneResponse {
	return ZoneResponse{
		ID:               z.ID,
		Name:             z.Name,
		Description:      z.Description,
		Capacity:         z.Capacity,
		AdditionalFee:    z.AdditionalFee,
		ForVehicleTypeID: z.ForVehicleTypeID,
		IsActive:         z.IsActive,
		CreatedAt:        z.CreatedAt,
		UpdatedAt:        z.UpdatedAt,
	}
}

func toGateResponse(g *Gate) GateResponse {
	return GateResponse{
		ID:              g.ID,
		ZoneID:          g.ZoneID,
		Name:            g.Name,
		GateType:        g.GateType,
		LocationDesc:    g.LocationDesc,
		GateToken:       g.GateToken,
		TokenLastUsedAt: g.TokenLastUsedAt,
		IsActive:        g.IsActive,
		CreatedAt:       g.CreatedAt,
		UpdatedAt:       g.UpdatedAt,
	}
}
