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

type RFIDCardResponse struct {
	ID            uuid.UUID  `json:"id"`
	CardUID       string     `json:"card_uid"`
	VehicleID     *uuid.UUID `json:"vehicle_id"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
	DeactivatedAt *time.Time `json:"deactivated_at"`
	DeactivatedBy *uuid.UUID `json:"deactivated_by"`
}

func toResponse(c *RFIDCard) RFIDCardResponse {
	return RFIDCardResponse{
		ID:            c.ID,
		CardUID:       c.CardUID,
		VehicleID:     c.VehicleID,
		IsActive:      c.IsActive,
		CreatedAt:     c.CreatedAt,
		DeactivatedAt: c.DeactivatedAt,
		DeactivatedBy: c.DeactivatedBy,
	}
}
