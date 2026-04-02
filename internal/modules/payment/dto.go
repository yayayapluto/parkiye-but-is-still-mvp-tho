package payment

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

type PayCashRequest struct {
	TransactionID uuid.UUID `json:"transaction_id" validate:"required"`
	CashTendered  int       `json:"cash_tendered" validate:"required,gt=0"`
}

type InitiateQRISRequest struct {
	TransactionID uuid.UUID `json:"transaction_id" validate:"required"`
}

// MidtransWebhookPayload is the raw body sent by Midtrans to our webhook endpoint.
// Fields are based on the official Midtrans notification docs.
type MidtransWebhookPayload struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"` // Midtrans internal transaction ID
	TransactionStatus string `json:"transaction_status"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
	FraudStatus       string `json:"fraud_status"`
	SettlementTime    string `json:"settlement_time"`
}

type RequestRefundRequest struct {
	PaymentID    uuid.UUID `json:"payment_id" validate:"required"`
	RefundAmount int       `json:"refund_amount" validate:"required,gt=0"`
	Reason       string    `json:"reason" validate:"required,min=10"`
}

type TransactionSummary struct {
	ID              uuid.UUID               `json:"id"`
	TransactionCode string                  `json:"transaction_code"`
	Status          types.TransactionStatus `json:"status"`
	CalculatedFee   *int                    `json:"calculated_fee"`
	ZoneID          uuid.UUID               `json:"zone_id"`
	EntryAt         time.Time               `json:"entry_at"`
	ExitAt          *time.Time              `json:"exit_at"`
}

type PaymentEnrichment struct {
	Transaction *TransactionSummary
}

type PaymentResponse struct {
	ID                    uuid.UUID           `json:"id"`
	TransactionID         uuid.UUID           `json:"transaction_id"`
	Method                types.PaymentMethod `json:"method"`
	Amount                int                 `json:"amount"`
	Status                types.PaymentStatus `json:"status"`
	HandledByUserID       *uuid.UUID          `json:"handled_by_user_id,omitempty"`
	CashTendered          *int                `json:"cash_tendered,omitempty"`
	CashChange            *int                `json:"cash_change,omitempty"`
	MidtransOrderID       *string             `json:"midtrans_order_id,omitempty"`
	MidtransTransactionID *string             `json:"midtrans_transaction_id,omitempty"`
	QRISString            *string             `json:"qris_string,omitempty"`
	QRISImageURL          *string             `json:"qris_image_url,omitempty"`
	QRISExpiresAt         *time.Time          `json:"qris_expires_at,omitempty"`
	MidtransStatus        *string             `json:"midtrans_status,omitempty"`
	PaidAt                *time.Time          `json:"paid_at,omitempty"`
	CreatedAt             time.Time           `json:"created_at"`
	UpdatedAt             time.Time           `json:"updated_at"`

	// Enriched fields
	Transaction *TransactionSummary `json:"transaction,omitempty"`
}

type RefundResponse struct {
	ID               uuid.UUID          `json:"id"`
	PaymentID        uuid.UUID          `json:"payment_id"`
	TransactionID    uuid.UUID          `json:"transaction_id"`
	RefundAmount     int                `json:"refund_amount"`
	Reason           string             `json:"reason"`
	Status           types.RefundStatus `json:"status"`
	RequestedBy      uuid.UUID          `json:"requested_by"`
	ApprovedBy       *uuid.UUID         `json:"approved_by,omitempty"`
	MidtransRefundID *string            `json:"midtrans_refund_id,omitempty"`
	ProcessedAt      *time.Time         `json:"processed_at,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

func toPaymentResponse(p *Payment, enr *PaymentEnrichment) PaymentResponse {
	res := PaymentResponse{
		ID:                    p.ID,
		TransactionID:         p.TransactionID,
		Method:                p.Method,
		Amount:                p.Amount,
		Status:                p.Status,
		HandledByUserID:       p.HandledByUserID,
		CashTendered:          p.CashTendered,
		CashChange:            p.CashChange,
		MidtransOrderID:       p.MidtransOrderID,
		MidtransTransactionID: p.MidtransTransactionID,
		QRISString:            p.QRISString,
		QRISImageURL:          p.QRISImageURL,
		QRISExpiresAt:         p.QRISExpiresAt,
		MidtransStatus:        p.MidtransStatus,
		PaidAt:                p.PaidAt,
		CreatedAt:             p.CreatedAt,
		UpdatedAt:             p.UpdatedAt,
	}

	if enr != nil {
		res.Transaction = enr.Transaction
	}

	return res
}

func toRefundResponse(r *Refund) RefundResponse {
	return RefundResponse{
		ID:               r.ID,
		PaymentID:        r.PaymentID,
		TransactionID:    r.TransactionID,
		RefundAmount:     r.RefundAmount,
		Reason:           r.Reason,
		Status:           r.Status,
		RequestedBy:      r.RequestedBy,
		ApprovedBy:       r.ApprovedBy,
		MidtransRefundID: r.MidtransRefundID,
		ProcessedAt:      r.ProcessedAt,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

