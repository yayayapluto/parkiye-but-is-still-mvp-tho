package payment

import (
	"context"

	"github.com/google/uuid"
)

type RepositoryPort interface {
	CreatePayment(ctx context.Context, p *Payment) error
	FindPaymentByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	FindPaymentByMidtransOrderID(ctx context.Context, orderID string) (*Payment, error)
	FindPaymentsByTransactionID(ctx context.Context, txID uuid.UUID) ([]Payment, error)
	UpdatePayment(ctx context.Context, p *Payment) error

	LogCallback(ctx context.Context, cb *MidtransCallback) error
	UpdateCallback(ctx context.Context, cb *MidtransCallback) error

	CreateRefund(ctx context.Context, r *Refund) error
	FindRefundByID(ctx context.Context, id uuid.UUID) (*Refund, error)
	FindRefundByPaymentID(ctx context.Context, paymentID uuid.UUID) ([]Refund, error)
	ListRefunds(ctx context.Context, page, pageSize int) ([]Refund, int64, error)
	UpdateRefund(ctx context.Context, r *Refund) error
}

type ServicePort interface {
	PayCash(ctx context.Context, req PayCashRequest, handledBy uuid.UUID) (*Payment, error)
	InitiateQRIS(ctx context.Context, req InitiateQRISRequest) (*Payment, error)
	HandleMidtransWebhook(ctx context.Context, rawBody []byte, payload MidtransWebhookPayload) error

	GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error)
	ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]Payment, error)

	RequestRefund(ctx context.Context, req RequestRefundRequest, requestedBy uuid.UUID) (*Refund, error)
	ApproveRefund(ctx context.Context, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error)
	RejectRefund(ctx context.Context, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error)
	ListRefunds(ctx context.Context, page, pageSize int) ([]Refund, int64, error)
}
