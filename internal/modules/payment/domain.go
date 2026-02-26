package payment

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"parkieee/pkg/types"
)

// Payment records a single payment attempt against a transaction.
// Multiple payments per transaction are allowed (e.g. QRIS expired → retry).
type Payment struct {
	ID                    uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID         uuid.UUID           `gorm:"type:uuid;not null;index"`
	Method                types.PaymentMethod `gorm:"type:varchar(20);not null"` // "cash"|"qris"
	Amount                int                 `gorm:"not null"`
	Status                types.PaymentStatus `gorm:"type:varchar(20);not null"` // "pending"|"completed"|"failed"|"expired"|"refunded"
	HandledByUserID       *uuid.UUID          `gorm:"type:uuid;index"`           // cashier user for cash; null for qris
	CashTendered          *int
	CashChange            *int
	MidtransOrderID       *string `gorm:"type:varchar(100);uniqueIndex"`
	MidtransTransactionID *string `gorm:"type:varchar(100)"`
	QRISUrl               *string `gorm:"type:text"`
	QRISExpiresAt         *time.Time
	MidtransStatus        *string `gorm:"type:varchar(50)"` // raw status string from Midtrans webhook
	PaidAt                *time.Time
	CreatedAt             time.Time `gorm:"autoCreateTime"`
	UpdatedAt             time.Time `gorm:"autoUpdateTime"`
}

func (Payment) TableName() string { return "payments" }

// MidtransCallback logs every Midtrans webhook hit, including duplicates and invalid signatures.
type MidtransCallback struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PaymentID       *uuid.UUID     `gorm:"type:uuid;index"`
	MidtransOrderID string         `gorm:"type:varchar(100);not null;index"`
	RawPayload      datatypes.JSON `gorm:"type:jsonb;not null"`
	SignatureValid  bool           `gorm:"not null"`
	Processed       bool           `gorm:"not null;default:false"`
	ProcessedAt     *time.Time
	ReceivedAt      time.Time `gorm:"not null;default:now()"`
	ErrorMessage    *string   `gorm:"type:text"`

	Payment *Payment `gorm:"foreignKey:PaymentID"`
}

func (MidtransCallback) TableName() string { return "midtrans_callbacks" }

// Refund records a refund request against a payment.
type Refund struct {
	ID               uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PaymentID        uuid.UUID          `gorm:"type:uuid;not null;index"`
	TransactionID    uuid.UUID          `gorm:"type:uuid;not null;index"`
	RefundAmount     int                `gorm:"not null"`
	Reason           string             `gorm:"type:text"`
	Status           types.RefundStatus `gorm:"type:varchar(20);not null"` // "pending"|"approved"|"processed"|"rejected"
	RequestedBy      uuid.UUID          `gorm:"type:uuid;not null"`
	ApprovedBy       *uuid.UUID         `gorm:"type:uuid"`
	MidtransRefundID *string            `gorm:"type:varchar(100)"`
	ProcessedAt      *time.Time
	CreatedAt        time.Time `gorm:"autoCreateTime"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime"`

	Payment *Payment `gorm:"foreignKey:PaymentID"`
}

func (Refund) TableName() string { return "refunds" }
