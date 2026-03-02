package transaction

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	ocrDomain "parkieee/internal/modules/ocr"
	"parkieee/pkg/types"
)

type RecordEntryRequest struct {
	EntryGateID uuid.UUID         `json:"entry_gate_id" form:"entry_gate_id" validate:"required"`
	EntryMethod types.EntryMethod `json:"entry_method"  form:"entry_method"  validate:"required,oneof=rfid qr"`
	RFIDCardUID string            `json:"rfid_card_uid" form:"rfid_card_uid"` // required when entry_method=rfid

	// Photo fields are NOT from JSON/form body — they are set by the handler
	// after saving the multipart file upload.
	EntryPhotoURL  string `json:"-" form:"-"`
	EntryPhotoPath string `json:"-" form:"-"`
}

type RecordExitRequest struct {
	ExitGateID    uuid.UUID        `json:"exit_gate_id"    form:"exit_gate_id"    validate:"required"`
	ExitMethod    types.ExitMethod `json:"exit_method"     form:"exit_method"     validate:"required,oneof=rfid qr override"`
	RFIDCardUID   string           `json:"rfid_card_uid"   form:"rfid_card_uid"`   // required when exit_method=rfid
	VehicleTypeID *uuid.UUID       `json:"vehicle_type_id" form:"vehicle_type_id"` // required when transaction has no vehicle linked yet

	// Photo fields are NOT from JSON/form body — they are set by the handler
	// after saving the multipart file upload.
	ExitPhotoURL  string `json:"-" form:"-"`
	ExitPhotoPath string `json:"-" form:"-"`
}

type CancelRequest struct {
	Reason string `json:"reason" validate:"required,min=5"`
}

type OCRPhotoSummary struct {
	PhotoType        string          `json:"photo_type"`         // "entry" or "exit"
	OCRDetectedPlate string          `json:"ocr_detected_plate"` // raw plate from OCR engine
	ActualPlate      string          `json:"actual_plate"`       // plate on the resolved vehicle record
	IsMatch          *bool           `json:"is_match"`           // null if vehicle not yet resolved
	Confidence       decimal.Decimal `json:"confidence"`
	OutputImageURL   *string         `json:"output_image_url"`  // public URL for annotated image
	OutputImagePath  *string         `json:"output_image_path"` // container path (debug reference)
	IsVerified       bool            `json:"is_verified"`
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
	EntryPhotoPath   *string                 `json:"entry_photo_path"`
	ExitPhotoURL     *string                 `json:"exit_photo_url"`
	ExitPhotoPath    *string                 `json:"exit_photo_path"`
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
	OCR              []OCRPhotoSummary       `json:"ocr,omitempty"`
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

func toResponse(t *Transaction, ocrResults []ocrDomain.OCRResultWithJob) TransactionResponse {
	var ocrSummary []OCRPhotoSummary
	for _, r := range ocrResults {
		ocrSummary = append(ocrSummary, OCRPhotoSummary{
			PhotoType:        string(r.PhotoType),
			OCRDetectedPlate: r.PlateDetected,
			ActualPlate:      r.ActualPlate,
			IsMatch:          r.IsMatch,
			Confidence:       r.Confidence,
			OutputImageURL:   r.OutputImageURL,
			OutputImagePath:  r.OutputImagePath,
			IsVerified:       r.IsVerified,
		})
	}
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
		EntryPhotoPath:   t.EntryPhotoPath,
		ExitPhotoURL:     t.ExitPhotoURL,
		ExitPhotoPath:    t.ExitPhotoPath,
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
		OCR:              ocrSummary,
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
