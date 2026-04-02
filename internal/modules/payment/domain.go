package payment

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"parkieee/pkg/types"
)

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

type PendingCashierRequest struct {
	TransactionID      string `json:"transaction_id"`
	TransactionCode    string `json:"transaction_code"`
	CalculatedFee      int    `json:"calculated_fee"`
	CashierRequestedAt string `json:"cashier_requested_at"`
}

type CashierEventType string

const (
	CashierEventCash     CashierEventType = "cash"
	CashierEventQRISFail CashierEventType = "qris_fail"
)

// CashierEvent dikirim kiosk ke kasir via SSE.
// GateID dipakai untuk routing ke kasir yang di-assign ke gate tersebut.
type CashierEvent struct {
	Type          CashierEventType `json:"type"`
	TransactionID string           `json:"transaction_id"`
	Amount        int              `json:"amount"`
	GateID        string           `json:"gate_id"`
	GateName      string           `json:"gate_name"`
	ZoneName      string           `json:"zone_name"`
}

type KioskEventType string

const (
	KioskEventDone   KioskEventType = "done"
	KioskEventCancel KioskEventType = "cancel"
)

type KioskEvent struct {
	Type          KioskEventType `json:"type"`
	TransactionID string         `json:"transaction_id"`
}

// CashierStatusResponse dikembalikan ke kiosk saat poll status kasir.
type CashierStatusResponse struct {
	Online   bool       `json:"online"`
	UserID   *uuid.UUID `json:"user_id"`
	UserName *string    `json:"user_name"`
}
