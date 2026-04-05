package payment

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	notifDomain "parkieee/internal/modules/notification"
	"parkieee/internal/modules/transaction"
	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/config"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

// cashierOnlineThreshold adalah durasi sejak lastSeenAt sebelum kasir dianggap offline.
const cashierOnlineThreshold = 10 * time.Second

type service struct {
	repo           RepositoryPort
	txSvc          transaction.ServicePort
	assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort
	midtrans       *midtransClient
	serverKey      string
	log            logger.Logger
	notifSvc       notifDomain.ServicePort

	cashierMu   sync.RWMutex
	cashierCh   map[string][]chan CashierEvent // key: userID kasir
	cashierSeen map[string]time.Time           // key: userID, value: lastSeenAt

	kioskMu sync.RWMutex
	kioskCh map[string]chan KioskEvent // key: txID
}

func NewService(
	repo RepositoryPort,
	txSvc transaction.ServicePort,
	assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort,
	cfg config.MidtransConfig,
	log logger.Logger,
	notifSvc notifDomain.ServicePort,
) ServicePort {
	return &service{
		repo:           repo,
		txSvc:          txSvc,
		assignmentRepo: assignmentRepo,
		midtrans:       newMidtransClient(cfg),
		serverKey:      cfg.ServerKey,
		log:            log,
		notifSvc:       notifSvc,
		cashierCh:      make(map[string][]chan CashierEvent),
		cashierSeen:    make(map[string]time.Time),
		kioskCh:        make(map[string]chan KioskEvent),
	}
}

func (s *service) PayCash(ctx context.Context, req PayCashRequest, handledBy uuid.UUID) (*Payment, error) {
	tx, err := s.txSvc.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	if tx.Status != types.TransactionStatusAwaitingPayment {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"Transaksi tidak dalam status menunggu pembayaran (status: %s)", tx.Status,
		))
	}
	if tx.CalculatedFee == nil {
		return nil, errors.New(errors.ErrValidation, "Transaksi belum memiliki tarif yang dihitung")
	}
	if req.CashTendered < *tx.CalculatedFee {
		return nil, errors.New(errors.ErrValidation, "Uang tunai yang diberikan kurang dari tarif yang harus dibayar")
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

	if err := s.txSvc.MarkPaidAndExited(ctx, req.TransactionID, types.TriggeredByCashier, &handledBy); err != nil {
		s.log.Error(ctx, "failed to mark transaction as paid and exited after cash payment", "tx_id", req.TransactionID, "error", err)
		return nil, err
	}

	// Notify cashier about successful payment
	go func() {
		_ = s.notifSvc.NotifyRole(context.Background(), "cashier", notifDomain.Notification{
			Type:     "payment",
			Title:    "Pembayaran Berhasil",
			Body:     fmt.Sprintf("Pembayaran tunai untuk transaksi di %s berhasil.", p.TransactionID),
			Metadata: datatypes.JSON([]byte(fmt.Sprintf(`{"payment_id":"%s","transaction_id":"%s"}`, p.ID, p.TransactionID))),
		})
	}()

	return p, nil
}

func (s *service) InitiateQRIS(ctx context.Context, req InitiateQRISRequest) (*Payment, error) {
	tx, err := s.txSvc.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	if tx.Status != types.TransactionStatusAwaitingPayment {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"Transaksi tidak dalam status menunggu pembayaran (status: %s)", tx.Status,
		))
	}
	if tx.CalculatedFee == nil {
		return nil, errors.New(errors.ErrValidation, "Transaksi belum memiliki tarif yang dihitung")
	}

	existing, err := s.repo.FindPaymentsByTransactionID(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}
	for i := range existing {
		if existing[i].Method == types.PaymentMethodQRIS && existing[i].Status == types.PaymentStatusPending {
			s.log.Debug(ctx, "QRIS payment already pending, returning existing", "payment_id", existing[i].ID, "tx_id", req.TransactionID)
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

	if err := s.repo.LogCallback(ctx, cb); err != nil {
		s.log.Error(ctx, "failed to log midtrans callback", "order_id", payload.OrderID, "error", err)
	}

	cb.SignatureValid = verifyMidtransSignature(
		payload.OrderID, payload.StatusCode, payload.GrossAmount, s.serverKey, payload.SignatureKey,
	)
	if err := s.repo.UpdateCallback(ctx, cb); err != nil {
		s.log.Error(ctx, "failed to update callback signature status", "error", err)
	}

	if !cb.SignatureValid {
		s.log.Warn(ctx, "invalid midtrans signature", "order_id", payload.OrderID)
		return nil
	}

	p, err := s.repo.FindPaymentByMidtransOrderID(ctx, payload.OrderID)
	if err != nil {
		s.log.Warn(ctx, "payment not found for webhook", "order_id", payload.OrderID)
		return nil
	}

	if p.Status == types.PaymentStatusCompleted {
		return nil
	}

	switch payload.TransactionStatus {
	case "settlement", "capture":
		if err := s.markQRISPaid(ctx, p, payload.TransactionStatus); err != nil {
			s.log.Error(ctx, "webhook: markQRISPaid failed", "order_id", payload.OrderID, "error", err)
		}
	case "expire", "cancel", "deny", "failure":
		p.Status = types.PaymentStatusFailed
		p.MidtransStatus = &payload.TransactionStatus
		if err := s.repo.UpdatePayment(ctx, p); err != nil {
			s.log.Error(ctx, "failed to update failed payment from webhook", "order_id", payload.OrderID, "error", err)
		}
		s.log.Info(ctx, "QRIS payment failed/expired via webhook", "order_id", payload.OrderID, "status", payload.TransactionStatus)
	default:
		return nil
	}

	now := time.Now()
	cb.Processed = true
	cb.ProcessedAt = &now
	if err := s.repo.UpdateCallback(ctx, cb); err != nil {
		s.log.Error(ctx, "failed to mark callback processed", "error", err)
	}

	return nil
}

func (s *service) markQRISPaid(ctx context.Context, p *Payment, midtransStatus string) error {
	if p.Status == types.PaymentStatusCompleted {
		return nil
	}
	now := time.Now()
	p.Status = types.PaymentStatusCompleted
	p.PaidAt = &now
	p.MidtransStatus = &midtransStatus
	if err := s.repo.UpdatePayment(ctx, p); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	if err := s.txSvc.MarkPaidAndExited(ctx, p.TransactionID, types.TriggeredByWebhook, nil); err != nil {
		s.log.Error(ctx, "markQRISPaid: failed to mark tx paid and exited", "tx_id", p.TransactionID, "error", err)
	}
	s.log.Info(ctx, "QRIS payment marked paid", "payment_id", p.ID, "tx_id", p.TransactionID, "source", midtransStatus)

	// Notify cashier about successful payment
	go func() {
		_ = s.notifSvc.NotifyRole(context.Background(), "cashier", notifDomain.Notification{
			Type:     "payment",
			Title:    "Pembayaran Berhasil",
			Body:     fmt.Sprintf("Pembayaran QRIS untuk transaksi di %s berhasil.", p.TransactionID),
			Metadata: datatypes.JSON([]byte(fmt.Sprintf(`{"payment_id":"%s","transaction_id":"%s"}`, p.ID, p.TransactionID))),
		})
	}()

	return nil
}

func (s *service) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	p, err := s.repo.FindPaymentByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get payment failed: not found", "payment_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "payment fetched", "payment_id", id, "tx_id", p.TransactionID, "status", p.Status)
	return p, nil
}

func (s *service) ListByTransaction(ctx context.Context, transactionID uuid.UUID) ([]Payment, error) {
	payments, err := s.repo.FindPaymentsByTransactionID(ctx, transactionID)
	if err != nil {
		s.log.Error(ctx, "list payments by transaction failed", "tx_id", transactionID, "error", err)
		return nil, err
	}
	s.log.Debug(ctx, "payments listed for transaction", "tx_id", transactionID, "count", len(payments))
	return payments, nil
}

func (s *service) RequestRefund(ctx context.Context, req RequestRefundRequest, requestedBy uuid.UUID) (*Refund, error) {
	p, err := s.repo.FindPaymentByID(ctx, req.PaymentID)
	if err != nil {
		s.log.Warn(ctx, "request refund failed: payment not found", "payment_id", req.PaymentID)
		return nil, err
	}
	if p.Status != types.PaymentStatusCompleted {
		return nil, errors.New(errors.ErrValidation, "Refund hanya dapat diajukan untuk pembayaran yang sudah selesai")
	}
	if req.RefundAmount > p.Amount {
		return nil, errors.New(errors.ErrValidation, "Jumlah refund melebihi jumlah pembayaran")
	}

	existing, err := s.repo.FindRefundByPaymentID(ctx, req.PaymentID)
	if err != nil {
		return nil, err
	}
	for i := range existing {
		if existing[i].Status == types.RefundStatusPending || existing[i].Status == types.RefundStatusProcessed {
			return nil, errors.New(errors.ErrConflict, "Sudah ada refund aktif untuk pembayaran ini")
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
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf("Refund tidak dalam status menunggu (status: %s)", ref.Status))
	}

	p, err := s.repo.FindPaymentByID(ctx, ref.PaymentID)
	if err != nil {
		return nil, err
	}

	if p.Method == types.PaymentMethodQRIS && p.MidtransTransactionID != nil {
		refundKey := uuid.New().String()
		if err := s.midtrans.refund(*p.MidtransTransactionID, refundKey, ref.RefundAmount, ref.Reason); err != nil {
			s.log.Error(ctx, "midtrans refund failed", "refund_id", refundID, "error", err)
			return nil, errors.Wrap(err, errors.ErrExternalService, "Gagal memproses refund melalui Midtrans")
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
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf("Refund tidak dalam status menunggu (status: %s)", ref.Status))
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
	refunds, total, err := s.repo.ListRefunds(ctx, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list refunds failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "refunds listed", "count", len(refunds), "total", total)
	return refunds, total, nil
}

func (s *service) ListPayments(ctx context.Context, page, pageSize int) ([]Payment, int64, error) {
	return s.repo.ListPayments(ctx, page, pageSize)
}

func (s *service) PollPaymentStatus(ctx context.Context, paymentID uuid.UUID) (*Payment, error) {
	p, err := s.repo.FindPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if p.Status == types.PaymentStatusPaid {
		s.log.Debug(ctx, "poll payment: already paid in DB, skipping Midtrans check", "payment_id", paymentID)
		return p, nil
	}
	if p.Method != types.PaymentMethodQRIS || p.MidtransOrderID == nil {
		return p, nil
	}

	status, err := s.midtrans.checkStatus(*p.MidtransOrderID)
	if err != nil {
		s.log.Warn(ctx, "poll midtrans status failed", "payment_id", paymentID, "error", err)
		return p, nil
	}

	if status.TransactionStatus == "settlement" || status.TransactionStatus == "capture" {
		if err := s.markQRISPaid(ctx, p, status.TransactionStatus); err != nil {
			s.log.Error(ctx, "poll: failed to mark payment paid", "payment_id", paymentID, "error", err)
			return p, nil
		}
		s.log.Info(ctx, "poll payment: payment settled via Midtrans poll", "payment_id", paymentID, "midtrans_status", status.TransactionStatus)
		return s.repo.FindPaymentByID(ctx, paymentID)
	}

	s.log.Debug(ctx, "poll payment: not yet settled", "payment_id", paymentID, "midtrans_status", status.TransactionStatus)
	return p, nil
}

func (s *service) SimulatePay(ctx context.Context, qrisImageURL string) error {
	if !s.midtrans.isSandbox() {
		return errors.New(errors.ErrValidation, "simulate pay hanya tersedia di sandbox")
	}
	if qrisImageURL == "" {
		return errors.New(errors.ErrValidation, "qris_image_url tidak boleh kosong")
	}
	s.log.Info(ctx, "[SIM] simulate QRIS pay", "qris_image_url", qrisImageURL)
	if err := s.midtrans.simulatePay(qrisImageURL); err != nil {
		s.log.Error(ctx, "[SIM] simulate pay failed", "error", err)
		return errors.Wrap(err, errors.ErrExternalService, "simulate pay gagal")
	}
	return nil
}

func (s *service) StampCashierRequested(ctx context.Context, txID uuid.UUID) error {
	now := time.Now()
	if err := s.repo.StampCashierRequested(ctx, txID, now); err != nil {
		s.log.Error(ctx, "stamp cashier_requested_at failed", "tx_id", txID, "error", err)
		return err
	}
	s.log.Info(ctx, "cashier_requested_at stamped", "tx_id", txID, "at", now)
	return nil
}

func (s *service) GetPendingCashierRequests(ctx context.Context, since string, cashierUserID uuid.UUID) ([]PendingCashierRequest, error) {
	txs, err := s.repo.FindPendingCashierRequests(ctx, since)
	if err != nil {
		s.log.Error(ctx, "get pending cashier requests failed", "since", since, "error", err)
		return nil, err
	}
	return txs, nil
}

// TouchCashierSeen update lastSeenAt kasir. Dipanggil tiap kali kasir hit endpoint apapun.
func (s *service) TouchCashierSeen(userID uuid.UUID) {
	s.cashierMu.Lock()
	s.cashierSeen[userID.String()] = time.Now()
	s.cashierMu.Unlock()
}

// GetCashierStatus cek apakah kasir yang di-assign ke gate sedang online.
// Lookup assignment dari DB, lalu cek lastSeenAt in-memory.
func (s *service) GetCashierStatus(gateID uuid.UUID) CashierStatusResponse {
	a, err := s.assignmentRepo.FindByGateID(context.Background(), gateID)
	if err != nil {
		return CashierStatusResponse{Online: false}
	}

	s.cashierMu.RLock()
	lastSeen, exists := s.cashierSeen[a.UserID.String()]
	s.cashierMu.RUnlock()

	online := exists && time.Since(lastSeen) <= cashierOnlineThreshold
	userID := a.UserID
	return CashierStatusResponse{
		Online: online,
		UserID: &userID,
	}
}

// NotifyCashier kirim event ke kasir yang di-assign ke gate tersebut.
// GateID di event dipakai untuk lookup cashier userID dari assignment.
func (s *service) NotifyCashier(cashierUserID uuid.UUID, event CashierEvent) error {
	key := cashierUserID.String()
	s.cashierMu.RLock()
	chans := append([]chan CashierEvent(nil), s.cashierCh[key]...)
	s.cashierMu.RUnlock()

	s.log.Info(context.Background(), "[SSE] NotifyCashier", "cashier_user_id", cashierUserID, "type", event.Type, "listeners", len(chans))
	for _, ch := range chans {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

// ListenCashier kasir subscribe per userID — hanya terima notif yang di-assign ke dia.
func (s *service) ListenCashier(cashierUserID uuid.UUID) (<-chan CashierEvent, func()) {
	key := cashierUserID.String()
	ch := make(chan CashierEvent, 4)

	s.cashierMu.Lock()
	s.cashierCh[key] = append(s.cashierCh[key], ch)
	s.cashierSeen[key] = time.Now()
	s.cashierMu.Unlock()

	cleanup := func() {
		s.cashierMu.Lock()
		defer s.cashierMu.Unlock()
		list := s.cashierCh[key]
		for i, c := range list {
			if c == ch {
				s.cashierCh[key] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(s.cashierCh[key]) == 0 {
			delete(s.cashierCh, key)
		}
		close(ch)
	}
	return ch, cleanup
}

func (s *service) NotifyKiosk(txID uuid.UUID, event KioskEvent) error {
	key := txID.String()
	s.kioskMu.RLock()
	ch, ok := s.kioskCh[key]
	s.kioskMu.RUnlock()
	if !ok {
		return nil
	}
	select {
	case ch <- event:
	default:
	}
	return nil
}

func (s *service) ListenKiosk(txID uuid.UUID) (<-chan KioskEvent, func()) {
	key := txID.String()
	ch := make(chan KioskEvent, 2)
	s.kioskMu.Lock()
	if old, ok := s.kioskCh[key]; ok {
		close(old)
	}
	s.kioskCh[key] = ch
	s.kioskMu.Unlock()
	cleanup := func() {
		s.kioskMu.Lock()
		defer s.kioskMu.Unlock()
		if s.kioskCh[key] == ch {
			delete(s.kioskCh, key)
			close(ch)
		}
	}
	return ch, cleanup
}

func verifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, incoming string) bool {
	raw := orderID + statusCode + grossAmount + serverKey
	h := sha512.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil)) == incoming
}

func (s *service) EnrichPayment(ctx context.Context, p *Payment, includes map[string]bool) *PaymentEnrichment {
	if includes == nil || !includes["transaction"] {
		return nil
	}

	enr := &PaymentEnrichment{}
	tx, err := s.txSvc.GetTransaction(ctx, p.TransactionID)
	if err == nil {
		enr.Transaction = &TransactionSummary{
			ID:              tx.ID,
			TransactionCode: tx.TransactionCode,
			Status:          tx.Status,
			CalculatedFee:   tx.CalculatedFee,
			ZoneID:          tx.ZoneID,
			EntryAt:         tx.EntryAt,
			ExitAt:          tx.ExitAt,
		}
	}

	return enr
}

func (s *service) EnrichPaymentList(ctx context.Context, payments []Payment, includes map[string]bool) map[uuid.UUID]PaymentEnrichment {
	if includes == nil || !includes["transaction"] {
		return nil
	}

	res := make(map[uuid.UUID]PaymentEnrichment)
	txIDs := make(map[uuid.UUID]bool)
	for _, p := range payments {
		txIDs[p.TransactionID] = true
	}

	txMap := make(map[uuid.UUID]*TransactionSummary)
	for id := range txIDs {
		tx, err := s.txSvc.GetTransaction(ctx, id)
		if err == nil {
			txMap[id] = &TransactionSummary{
				ID:              tx.ID,
				TransactionCode: tx.TransactionCode,
				Status:          tx.Status,
				CalculatedFee:   tx.CalculatedFee,
				ZoneID:          tx.ZoneID,
				EntryAt:         tx.EntryAt,
				ExitAt:          tx.ExitAt,
			}
		}
	}

	for _, p := range payments {
		if tx, ok := txMap[p.TransactionID]; ok {
			res[p.ID] = PaymentEnrichment{Transaction: tx}
		}
	}

	return res
}

