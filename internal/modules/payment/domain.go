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
	ID                    uuid.UUID           `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID         uuid.UUID           `gorm:"column:transaction_id;type:uuid;not null;index"`
	Method                types.PaymentMethod `gorm:"column:method;type:varchar(20);not null"`
	Amount                int                 `gorm:"column:amount;not null"`
	Status                types.PaymentStatus `gorm:"column:status;type:varchar(20);not null"`
	HandledByUserID       *uuid.UUID          `gorm:"column:handled_by_user_id;type:uuid;index"`
	CashTendered          *int                `gorm:"column:cash_tendered"`
	CashChange            *int                `gorm:"column:cash_change"`
	MidtransOrderID       *string             `gorm:"column:midtrans_order_id;type:varchar(100);uniqueIndex"`
	MidtransTransactionID *string             `gorm:"column:midtrans_transaction_id;type:varchar(100)"`
	QRISString            *string             `gorm:"column:qris_string;type:text"`
	QRISImageURL          *string             `gorm:"column:qris_image_url;type:text"`
	QRISExpiresAt         *time.Time          `gorm:"column:qris_expires_at;type:timestamptz"`
	MidtransStatus        *string             `gorm:"column:midtrans_status;type:varchar(50)"`
	PaidAt                *time.Time          `gorm:"column:paid_at;type:timestamptz"`
	CreatedAt             time.Time           `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time           `gorm:"column:updated_at;autoUpdateTime"`
}

func (Payment) TableName() string { return "payments" }

// MidtransCallback logs every Midtrans webhook hit, including duplicates and invalid signatures.
// Always written before any processing — used for audit and replay.
type MidtransCallback struct {
	ID              uuid.UUID      `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	PaymentID       *uuid.UUID     `gorm:"column:payment_id;type:uuid;index"`
	MidtransOrderID string         `gorm:"column:midtrans_order_id;type:varchar(100);not null;index"`
	RawPayload      datatypes.JSON `gorm:"column:raw_payload;type:jsonb;not null"`
	SignatureValid  bool           `gorm:"column:signature_valid;not null"`
	Processed       bool           `gorm:"column:processed;not null;default:false"`
	ProcessedAt     *time.Time     `gorm:"column:processed_at;type:timestamptz"`
	ReceivedAt      time.Time      `gorm:"column:received_at;not null;default:now()"`
	ErrorMessage    *string        `gorm:"column:error_message;type:text"`
}

func (MidtransCallback) TableName() string { return "midtrans_callbacks" }

// Refund records a refund request against a completed payment.
type Refund struct {
	ID               uuid.UUID          `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	PaymentID        uuid.UUID          `gorm:"column:payment_id;type:uuid;not null;index"`
	TransactionID    uuid.UUID          `gorm:"column:transaction_id;type:uuid;not null;index"`
	RefundAmount     int                `gorm:"column:refund_amount;not null"`
	Reason           string             `gorm:"column:reason;type:text"`
	Status           types.RefundStatus `gorm:"column:status;type:varchar(20);not null"`
	RequestedBy      uuid.UUID          `gorm:"column:requested_by;type:uuid;not null"`
	ApprovedBy       *uuid.UUID         `gorm:"column:approved_by;type:uuid"`
	MidtransRefundID *string            `gorm:"column:midtrans_refund_id;type:varchar(100)"`
	ProcessedAt      *time.Time         `gorm:"column:processed_at;type:timestamptz"`
	CreatedAt        time.Time          `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time          `gorm:"column:updated_at;autoUpdateTime"`
}

func (Refund) TableName() string { return "refunds" }
