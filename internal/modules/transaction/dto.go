package transaction

import (
	"time"

	"github.com/google/uuid"

	"parkieee/pkg/types"
)

type RecordEntryRequest struct {
	EntryGateID   uuid.UUID         `json:"entry_gate_id"   validate:"required"`
	EntryMethod   types.EntryMethod `json:"entry_method"    validate:"required,oneof=rfid qr"`
	RFIDCardUID   string            `json:"rfid_card_uid"`   // required when entry_method=rfid
	EntryPhotoURL string            `json:"entry_photo_url"` // optional
}

type RecordExitRequest struct {
	ExitGateID    uuid.UUID        `json:"exit_gate_id"   validate:"required"`
	ExitMethod    types.ExitMethod `json:"exit_method"    validate:"required,oneof=rfid qr override"`
	RFIDCardUID   string           `json:"rfid_card_uid"`   // required when exit_method=rfid
	VehicleTypeID *uuid.UUID       `json:"vehicle_type_id"` // required when transaction has no vehicle linked yet
}

type CancelRequest struct {
	Reason string `json:"reason" validate:"required,min=5"`
}

type TransactionResponse struct {
	ID               uuid.UUID               `json:"id"`
	TransactionCode  string                  `json:"transaction_code"`
	EntryGateID      uuid.UUID               `json:"entry_gate_id"`
	EntryMethod      types.EntryMethod       `json:"entry_method"`
	RFIDCardID       *uuid.UUID              `json:"rfid_card_id"`
	EntryQRCode      *string                 `json:"entry_qr_code"`
	EntryQRCodeImage *string                 `json:"entry_qr_code_image"`
	EntryAt          time.Time               `json:"entry_at"`
	EntryPhotoURL    *string                 `json:"entry_photo_url"`
	ExitGateID       *uuid.UUID              `json:"exit_gate_id"`
	ExitMethod       *types.ExitMethod       `json:"exit_method"`
	ExitAt           *time.Time              `json:"exit_at"`
	VehicleID        *uuid.UUID              `json:"vehicle_id"`
	FeeConfigID      *uuid.UUID              `json:"fee_config_id"`
	CalculatedFee    *int                    `json:"calculated_fee"`
	HolidayRateID    *uuid.UUID              `json:"holiday_rate_id"`
	Status           types.TransactionStatus `json:"status"`
	ReceiptPrinted   bool                    `json:"receipt_printed"`
	ZoneID           uuid.UUID               `json:"zone_id"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

type TransactionLogResponse struct {
	ID                uuid.UUID              `json:"id"`
	TransactionID     uuid.UUID              `json:"transaction_id"`
	FromStatus        *string                `json:"from_status"`
	ToStatus          string                 `json:"to_status"`
	Event             types.TransactionEvent `json:"event"`
	TriggeredBy       types.TriggeredBy      `json:"triggered_by"`
	TriggeredByUserID *uuid.UUID             `json:"triggered_by_user_id"`
	Note              string                 `json:"note"`
	CreatedAt         time.Time              `json:"created_at"`
}

func toResponse(t *Transaction) TransactionResponse {
	return TransactionResponse{
		ID:               t.ID,
		TransactionCode:  t.TransactionCode,
		EntryGateID:      t.EntryGateID,
		EntryMethod:      t.EntryMethod,
		RFIDCardID:       t.RFIDCardID,
		EntryQRCode:      t.EntryQRCode,
		EntryQRCodeImage: t.EntryQRCodeImage,
		EntryAt:          t.EntryAt,
		EntryPhotoURL:    t.EntryPhotoURL,
		ExitGateID:       t.ExitGateID,
		ExitMethod:       t.ExitMethod,
		ExitAt:           t.ExitAt,
		VehicleID:        t.VehicleID,
		FeeConfigID:      t.FeeConfigID,
		CalculatedFee:    t.CalculatedFee,
		HolidayRateID:    t.HolidayRateID,
		Status:           t.Status,
		ReceiptPrinted:   t.ReceiptPrinted,
		ZoneID:           t.ZoneID,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

func toLogResponse(l *TransactionLog) TransactionLogResponse {
	return TransactionLogResponse{
		ID:                l.ID,
		TransactionID:     l.TransactionID,
		FromStatus:        l.FromStatus,
		ToStatus:          l.ToStatus,
		Event:             l.Event,
		TriggeredBy:       l.TriggeredBy,
		TriggeredByUserID: l.TriggeredByUserID,
		Note:              l.Note,
		CreatedAt:         l.CreatedAt,
	}
}
