package rfid

import (
	"time"

	"github.com/google/uuid"
)

type RegisterOrGetRequest struct {
	CardUID string `json:"card_uid" validate:"required,min=4,max=64"`
}

type LinkVehicleRequest struct {
	VehicleID uuid.UUID `json:"vehicle_id" validate:"required"`
}

type LinkedVehicle struct {
	ID          uuid.UUID `json:"id"`
	PlateNumber string    `json:"plate_number"`
	VehicleType struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"vehicle_type"`
}

type RFIDCardEnrichment struct {
	Vehicle *LinkedVehicle
}

type RFIDCardResponse struct {
	ID            uuid.UUID  `json:"id"`
	CardUID       string     `json:"card_uid"`
	VehicleID     *uuid.UUID `json:"vehicle_id"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
	DeactivatedAt *time.Time `json:"deactivated_at"`
	DeactivatedBy *uuid.UUID `json:"deactivated_by"`

	// Enriched fields
	Vehicle *LinkedVehicle `json:"vehicle,omitempty"`
}

func toResponse(c *RFIDCard, enr *RFIDCardEnrichment) RFIDCardResponse {
	res := RFIDCardResponse{
		ID:            c.ID,
		CardUID:       c.CardUID,
		VehicleID:     c.VehicleID,
		IsActive:      c.IsActive,
		CreatedAt:     c.CreatedAt,
		DeactivatedAt: c.DeactivatedAt,
		DeactivatedBy: c.DeactivatedBy,
	}

	if enr != nil {
		res.Vehicle = enr.Vehicle
	}

	return res
}

type ListRFIDFilter struct {
	CardUID   string
	IsActive  *bool
	Search    string
	DateFrom  *time.Time
	DateTo    *time.Time
	SortBy    string
	SortOrder string
}

