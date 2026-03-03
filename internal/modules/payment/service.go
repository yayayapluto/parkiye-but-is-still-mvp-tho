package payment

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"parkieee/internal/modules/transaction"
	"parkieee/pkg/config"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

type service struct {
	repo      RepositoryPort
	txSvc     transaction.ServicePort
	midtrans  *midtransClient
	serverKey string
	log       logger.Logger
}

func NewService(
	repo RepositoryPort,
	txSvc transaction.ServicePort,
	cfg config.MidtransConfig,
	log logger.Logger,
) ServicePort {
	return &service{
		repo:      repo,
		txSvc:     txSvc,
		midtrans:  newMidtransClient(cfg),
		serverKey: cfg.ServerKey,
		log:       log,
	}
}

func (s *service) PayCash(ctx context.Context, req PayCashRequest, handledBy uuid.UUID) (*Payment, error) {
	tx, err := s.txSvc.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	if tx.Status != types.TransactionStatusAwaitingPayment {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"transaction is not awaiting payment (status: %s)", tx.Status,
		))
	}
	if tx.CalculatedFee == nil {
		return nil, errors.New(errors.ErrValidation, "transaction has no calculated fee")
	}
	if req.CashTendered < *tx.CalculatedFee {
		return nil, errors.New(errors.ErrValidation, "cash tendered is less than the required fee")
	}

	cashChange := req.CashTendered - *tx.CalculatedFee
	now := time.Now()

	p := &Payment{
		TransactionID:   req.TransactionID,
		Method:          types.PaymentMethodCash,
		Amount:          *tx.CalculatedFee,
		Status:          types.PaymentStatusCompleted,
		HandledByUserID: &handledBy,
		CashTendered:    &req.CashTendered,
		CashChange:      &cashChange,
		PaidAt:          &now,
	}

	if err := s.repo.CreatePayment(ctx, p); err != nil {
		s.log.Error(ctx, "failed to create cash payment record", "tx_id", req.TransactionID, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "cash payment recorded", "payment_id", p.ID, "tx_id", req.TransactionID, "amount", p.Amount, "tendered", req.CashTendered, "change", cashChange, "handled_by", handledBy)

	if err := s.txSvc.MarkPaid(ctx, req.TransactionID, types.TriggeredByCashier, &handledBy); err != nil {
		s.log.Error(ctx, "failed to mark transaction as paid after cash payment",
			"tx_id", req.TransactionID, "payment_id", p.ID, "error", err)
		return nil, err
	}

	if err := s.txSvc.MarkExited(ctx, req.TransactionID, types.TriggeredByCashier); err != nil {
		s.log.Error(ctx, "failed to mark transaction as exited after cash payment",
			"tx_id", req.TransactionID, "payment_id", p.ID, "error", err)
		return nil, err
	}

	return p, nil
}

func (s *service) InitiateQRIS(ctx context.Context, req InitiateQRISRequest) (*Payment, error) {
	tx, err := s.txSvc.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	if tx.Status != types.TransactionStatusAwaitingPayment {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"transaction is not awaiting payment (status: %s)", tx.Status,
		))
	}
	if tx.CalculatedFee == nil {
		return nil, errors.New(errors.ErrValidation, "transaction has no calculated fee")
	}

	// Idempotent: return existing pending QRIS payment if exists
	existing, err := s.repo.FindPaymentsByTransactionID(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	for i := range existing {
		if existing[i].Method == types.PaymentMethodQRIS && existing[i].Status == types.PaymentStatusPending {
			return &existing[i], nil
		}
	}

	orderID := "PKR-" + uuid.New().String()
	resp, err := s.midtrans.chargeQRIS(orderID, *tx.CalculatedFee)
	if err != nil {
		s.log.Error(ctx, "midtrans QRIS charge failed", "tx_id", req.TransactionID, "error", err)
		return nil, errors.Wrap(err, errors.ErrExternalService, "failed to initiate QRIS payment")
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	qrisString := resp.QRString

	// Pick image URL from actions
	imageURL := ""
	for _, action := range resp.Actions {
		if action.Name == "generate-qr-code" {
			imageURL = action.URL
			break
		}
	}
	if imageURL == "" && len(resp.Actions) > 0 {
		imageURL = resp.Actions[0].URL
	}

	p := &Payment{
		TransactionID:         req.TransactionID,
		Method:                types.PaymentMethodQRIS,
		Amount:                *tx.CalculatedFee,
		Status:                types.PaymentStatusPending,
		MidtransOrderID:       &orderID,
		MidtransTransactionID: &resp.TransactionID,
		QRISString:            &qrisString,
		QRISImageURL:          &imageURL,
		QRISExpiresAt:         &expiresAt,
	}

	if err := s.repo.CreatePayment(ctx, p); err != nil {
		s.log.Error(ctx, "failed to create QRIS payment record", "tx_id", req.TransactionID, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "QRIS payment initiated", "payment_id", p.ID, "tx_id", req.TransactionID, "order_id", orderID, "amount", p.Amount)

	return p, nil
}

func (s *service) HandleMidtransWebhook(ctx context.Context, rawBody []byte, payload MidtransWebhookPayload) error {
	cb := &MidtransCallback{
		MidtransOrderID: payload.OrderID,
		RawPayload:      datatypes.JSON(rawBody),
		SignatureValid:  false,
		ReceivedAt:      time.Now(),
	}

	// Always log first — audit trail
	if err := s.repo.LogCallback(ctx, cb); err != nil {
		s.log.Error(ctx, "failed to log midtrans callback", "order_id", payload.OrderID, "error", err)
	}

	// Verify signature
	cb.SignatureValid = verifyMidtransSignature(
		payload.OrderID, payload.StatusCode, payload.GrossAmount, s.serverKey, payload.SignatureKey,
	)
	if err := s.repo.UpdateCallback(ctx, cb); err != nil {
		s.log.Error(ctx, "failed to update callback signature status", "error", err)
	}

	if !cb.SignatureValid {
		s.log.Warn(ctx, "invalid midtrans signature", "order_id", payload.OrderID)
		return nil // Always HTTP 200 to Midtrans
	}

	p, err := s.repo.FindPaymentByMidtransOrderID(ctx, payload.OrderID)
	if err != nil {
		s.log.Warn(ctx, "payment not found for webhook", "order_id", payload.OrderID)
		return nil
	}

	// Idempotent
	if p.Status == types.PaymentStatusCompleted {
		return nil
	}

	switch payload.TransactionStatus {
	case "settlement":
		now := time.Now()
		p.Status = types.PaymentStatusCompleted
		p.PaidAt = &now
	case "expire", "cancel", "deny", "failure":
		p.Status = types.PaymentStatusFailed
	default:
		return nil // pending, dll — ignore
	}

	p.MidtransStatus = &payload.TransactionStatus
	if err := s.repo.UpdatePayment(ctx, p); err != nil {
		s.log.Error(ctx, "failed to update payment from webhook", "order_id", payload.OrderID, "error", err)
		return nil
	}

	if p.Status == types.PaymentStatusCompleted {
		s.log.Info(ctx, "QRIS payment settled via webhook", "order_id", payload.OrderID, "tx_id", p.TransactionID)
		if err := s.txSvc.MarkPaid(ctx, p.TransactionID, types.TriggeredByWebhook, nil); err != nil {
			s.log.Error(ctx, "failed to mark transaction paid from webhook", "tx_id", p.TransactionID, "error", err)
		} else if err := s.txSvc.MarkExited(ctx, p.TransactionID, types.TriggeredByWebhook); err != nil {
			s.log.Error(ctx, "failed to mark transaction exited from webhook", "tx_id", p.TransactionID, "error", err)
		}
	} else {
		s.log.Info(ctx, "QRIS payment failed/expired via webhook", "order_id", payload.OrderID, "status", payload.TransactionStatus, "tx_id", p.TransactionID)
	}

	now := time.Now()
	cb.Processed = true
	cb.ProcessedAt = &now
	if err := s.repo.UpdateCallback(ctx, cb); err != nil {
		s.log.Error(ctx, "failed to mark callback processed", "error", err)
	}

	return nil
}

func (s *service) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	return s.repo.FindPaymentByID(ctx, id)
}

func (s *service) ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]Payment, error) {
	return s.repo.FindPaymentsByTransactionID(ctx, transactionID)
}

func (s *service) RequestRefund(ctx context.Context, req RequestRefundRequest, requestedBy uuid.UUID) (*Refund, error) {
	p, err := s.repo.FindPaymentByID(ctx, req.PaymentID)
	if err != nil {
		s.log.Warn(ctx, "request refund failed: payment not found", "payment_id", req.PaymentID)
		return nil, err
	}
	if p.Status != types.PaymentStatusCompleted {
		s.log.Warn(ctx, "request refund rejected: payment not completed", "payment_id", req.PaymentID, "status", p.Status)
		return nil, errors.New(errors.ErrValidation, "refund can only be requested for completed payments")
	}
	if req.RefundAmount > p.Amount {
		s.log.Warn(ctx, "request refund rejected: amount exceeds payment", "payment_id", req.PaymentID, "refund_amount", req.RefundAmount, "payment_amount", p.Amount)
		return nil, errors.New(errors.ErrValidation, "refund amount exceeds payment amount")
	}

	existing, err := s.repo.FindRefundByPaymentID(ctx, req.PaymentID)
	if err != nil {
		return nil, err
	}
	for i := range existing {
		if existing[i].Status == types.RefundStatusPending || existing[i].Status == types.RefundStatusProcessed {
			return nil, errors.New(errors.ErrConflict, "there is already an active refund for this payment")
		}
	}

	ref := &Refund{
		PaymentID:     req.PaymentID,
		TransactionID: p.TransactionID,
		RefundAmount:  req.RefundAmount,
		Reason:        req.Reason,
		Status:        types.RefundStatusPending,
		RequestedBy:   requestedBy,
	}
	if err := s.repo.CreateRefund(ctx, ref); err != nil {
		s.log.Error(ctx, "failed to create refund request", "payment_id", req.PaymentID, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "refund requested", "refund_id", ref.ID, "payment_id", req.PaymentID, "amount", req.RefundAmount, "requested_by", requestedBy)
	return ref, nil
}

func (s *service) ApproveRefund(ctx context.Context, refundID uuid.UUID, approvedBy uuid.UUID) (*Refund, error) {
	ref, err := s.repo.FindRefundByID(ctx, refundID)
	if err != nil {
		return nil, err
	}
	if ref.Status != types.RefundStatusPending {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"refund is not pending (status: %s)", ref.Status,
		))
	}

	p, err := s.repo.FindPaymentByID(ctx, ref.PaymentID)
	if err != nil {
		return nil, err
	}

	if p.Method == types.PaymentMethodQRIS && p.MidtransTransactionID != nil {
		refundKey := uuid.New().String()
		if err := s.midtrans.refund(*p.MidtransTransactionID, refundKey, ref.RefundAmount, ref.Reason); err != nil {
			s.log.Error(ctx, "midtrans refund failed", "refund_id", refundID, "error", err)
			return nil, errors.Wrap(err, errors.ErrExternalService, "failed to process refund via Midtrans")
		}
		ref.MidtransRefundID = &refundKey
	}

	now := time.Now()
	ref.Status = types.RefundStatusProcessed
	ref.ApprovedBy = &approvedBy
	ref.ProcessedAt = &now

	if err := s.repo.UpdateRefund(ctx, ref); err != nil {
		s.log.Error(ctx, "failed to update refund after approval", "refund_id", refundID, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "refund approved", "refund_id", refundID, "payment_id", ref.PaymentID, "amount", ref.RefundAmount, "approved_by", approvedBy)
	return ref, nil
}

func (s *service) RejectRefund(ctx context.Context, refundID uuid.UUID, rejectedBy uuid.UUID) (*Refund, error) {
	ref, err := s.repo.FindRefundByID(ctx, refundID)
	if err != nil {
		s.log.Warn(ctx, "reject refund failed: refund not found", "refund_id", refundID)
		return nil, err
	}
	if ref.Status != types.RefundStatusPending {
		s.log.Warn(ctx, "reject refund failed: not pending", "refund_id", refundID, "status", ref.Status)
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"refund is not pending (status: %s)", ref.Status,
		))
	}

	ref.Status = types.RefundStatusRejected
	ref.ApprovedBy = &rejectedBy

	if err := s.repo.UpdateRefund(ctx, ref); err != nil {
		s.log.Error(ctx, "failed to update refund after rejection", "refund_id", refundID, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "refund rejected", "refund_id", refundID, "payment_id", ref.PaymentID, "rejected_by", rejectedBy)
	return ref, nil
}

func (s *service) ListRefunds(ctx context.Context, page, pageSize int) ([]Refund, int64, error) {
	return s.repo.ListRefunds(ctx, page, pageSize)
}

func verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, incoming string) bool {
	raw := orderID + statusCode + grossAmount + serverKey
	h := sha512.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil)) == incoming
}
