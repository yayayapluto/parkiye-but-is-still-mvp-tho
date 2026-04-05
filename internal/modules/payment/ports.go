package payment

import (
	"context"
	"time"

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
	ListPayments(ctx context.Context, page, pageSize int) ([]Payment, int64, error)
	// Polling Cashier
	StampCashierRequested(ctx context.Context, txID uuid.UUID, requestedAt time.Time) error
	FindPendingCashierRequests(ctx context.Context, since string) ([]PendingCashierRequest, error)
}

type ServicePort interface {
	PayCash(ctx context.Context, req PayCashRequest, handledBy uuid.UUID) (*Payment, error)
	InitiateQRIS(ctx context.Context, req InitiateQRISRequest) (*Payment, error)
	HandleMidtransWebhook(ctx context.Context, rawBody []byte, payload MidtransWebhookPayload) error

	GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error)
	PollPaymentStatus(ctx context.Context, paymentID uuid.UUID) (*Payment, error)
	ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]Payment, error)

	RequestRefund(ctx context.Context, req RequestRefundRequest, requestedBy uuid.UUID) (*Refund, error)
	ApproveRefund(ctx context.Context, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error)
	RejectRefund(ctx context.Context, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error)
	ListRefunds(ctx context.Context, page, pageSize int) ([]Refund, int64, error)
	ListPayments(ctx context.Context, page, pageSize int) ([]Payment, int64, error)
	// SSE Cashier — routing per userID (bukan broadcast)
	NotifyCashier(cashierUserID uuid.UUID, event CashierEvent) error
	ListenCashier(cashierUserID uuid.UUID) (<-chan CashierEvent, func())
	NotifyKiosk(txID uuid.UUID, event KioskEvent) error
	ListenKiosk(txID uuid.UUID) (<-chan KioskEvent, func())

	// Polling Cashier (Cloudflare SSE workaround)
	StampCashierRequested(ctx context.Context, txID uuid.UUID) error
	GetPendingCashierRequests(ctx context.Context, since string, cashierUserID uuid.UUID) ([]PendingCashierRequest, error)

	// Cashier online detection — dipakai kiosk sebelum tampilkan opsi tunai
	TouchCashierSeen(userID uuid.UUID)
	GetCashierStatus(gateID uuid.UUID) CashierStatusResponse

	// Sandbox only
	SimulatePay(ctx context.Context, qrisImageURL string) error

	// Enrichment
	EnrichPayment(ctx context.Context, p *Payment, includes map[string]bool) *PaymentEnrichment
	EnrichPaymentList(ctx context.Context, payments []Payment, includes map[string]bool) map[uuid.UUID]PaymentEnrichment
}
