package transaction

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"parkieee/pkg/types"
)

// Transaction is the core parking session record.
// format: PKR-YYYYMMDD-{sequence} e.g. PKR-20260222-00042
type Transaction struct {
	ID               uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionCode  string            `gorm:"type:varchar(50);uniqueIndex;not null"`
	EntryGateID      uuid.UUID         `gorm:"type:uuid;not null;index"`
	EntryMethod      types.EntryMethod `gorm:"type:varchar(10);not null"`           // "rfid"|"qr"
	RFIDCardID       *uuid.UUID        `gorm:"type:uuid;index;column:rfid_card_id"` // null if entry_method = "qr"
	EntryQRCode      *string           `gorm:"type:varchar(255);uniqueIndex"`       // null if entry_method = "rfid"
	EntryQRCodeImage *string           `gorm:"type:text"`                           // base64 PNG of the QR code; null if entry_method = "rfid"
	EntryAt          time.Time         `gorm:"not null"`
	EntryPhotoURL    *string           `gorm:"type:text"` // null if camera failed
	EntryPhotoPath   *string           `gorm:"type:text"` // absolute path on shared Docker volume for OCR
	ExitGateID       *uuid.UUID        `gorm:"type:uuid;index"`
	ExitMethod       *types.ExitMethod `gorm:"type:varchar(10)"` // "rfid"|"qr"|"override"
	ExitAt           *time.Time
	ExitPhotoURL     *string    `gorm:"type:text"`       // null if camera failed
	ExitPhotoPath    *string    `gorm:"type:text"`       // absolute path on shared Docker volume for OCR
	VehicleID        *uuid.UUID `gorm:"type:uuid;index"` // null until OCR resolves or operator sets manually
	FeeConfigID      *uuid.UUID `gorm:"type:uuid"`       // snapshot of config used at time of calculation
	CalculatedFee    *int
	HolidayRateID    *uuid.UUID              `gorm:"type:uuid"`                 // null if no holiday rate applied
	Status           types.TransactionStatus `gorm:"type:varchar(20);not null"` // "open"|"awaiting_payment"|"paid"|"exited"|"overridden"|"cancelled"
	ReceiptPrinted   bool                    `gorm:"not null;default:false"`
	ReceiptPrintedAt *time.Time
	ZoneID           uuid.UUID `gorm:"type:uuid;not null;index"`
	CreatedAt        time.Time `gorm:"autoCreateTime"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime"`
}

func (Transaction) TableName() string { return "transactions" }

// TransactionLog is append-only; every status transition is recorded.
type TransactionLog struct {
	ID                uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID     uuid.UUID              `gorm:"type:uuid;not null;index"`
	FromStatus        *string                `gorm:"type:varchar(20)"`
	ToStatus          string                 `gorm:"type:varchar(20);not null"`
	Event             types.TransactionEvent `gorm:"type:varchar(50);not null"`
	TriggeredBy       types.TriggeredBy      `gorm:"type:varchar(20);not null"` // "system"|"operator"|"cashier"|"webhook"
	TriggeredByUserID *uuid.UUID             `gorm:"type:uuid;index"`           // null if triggered_by = "system"/"webhook"
	Note              string                 `gorm:"type:text"`
	Metadata          datatypes.JSON         `gorm:"type:jsonb"`
	CreatedAt         time.Time              `gorm:"not null;default:now()"`

	Transaction *Transaction `gorm:"foreignKey:TransactionID"`
}

func (TransactionLog) TableName() string { return "transaction_logs" }

// UnclosedTransactionFlag marks transactions that never exited.
type UnclosedTransactionFlag struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID  uuid.UUID      `gorm:"type:uuid;not null;index"`
	FlaggedAt      time.Time      `gorm:"not null;default:now()"`
	FlagType       types.FlagType `gorm:"type:varchar(30);not null"` // "overnight"|"multi_day"|"suspicious_duration"
	FlagReason     string         `gorm:"type:varchar(100)"`
	Resolved       bool           `gorm:"not null;default:false"`
	ResolvedAt     *time.Time
	ResolvedBy     *uuid.UUID `gorm:"type:uuid"`
	ResolutionNote string     `gorm:"type:text"`

	Transaction *Transaction `gorm:"foreignKey:TransactionID"`
}

func (UnclosedTransactionFlag) TableName() string { return "unclosed_transaction_flags" }
