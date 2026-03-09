package payment_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"parkieee/internal/modules/payment"
	paymentmocks "parkieee/internal/modules/payment/mocks"
	txmocks "parkieee/internal/modules/transaction/mocks"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
)

const testServerKey = "SB-Mid-server-test-key-12345"

type noopLogger struct{}

func (noopLogger) Debug(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Info(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Warn(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Error(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Fatal(_ context.Context, _ string, _ ...any) {}
func (n noopLogger) With(_ ...any) logger.Logger               { return n }
func (n noopLogger) WithGroup(_ string) logger.Logger          { return n }

var _ logger.Logger = noopLogger{}

func newSvc(t *testing.T) (payment.ServicePort, *paymentmocks.MockRepositoryPort) {
	repo := paymentmocks.NewMockRepositoryPort(t)
	txSvc := txmocks.NewMockServicePort(t)

	svc := payment.NewService(
		repo,
		txSvc,
		config.MidtransConfig{ServerKey: testServerKey, Env: "sandbox"},
		noopLogger{},
	)
	return svc, repo
}

// midtransSignature replicates verifyMidtransSignature logic for building valid test payloads.
func midtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	raw := orderID + statusCode + grossAmount + serverKey
	h := sha512.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}

func TestHandleMidtransWebhook_InvalidSignature_StillReturnsNil(t *testing.T) {
	svc, repo := newSvc(t)

	payload := payment.MidtransWebhookPayload{
		OrderID:      "order-001",
		StatusCode:   "200",
		GrossAmount:  "15000.00",
		SignatureKey: "wrong-signature",
	}
	rawBody, _ := json.Marshal(payload)

	// Callback harus selalu di-log terlebih dahulu, bahkan untuk signature invalid
	repo.EXPECT().
		LogCallback(context.Background(), mock.MatchedBy(func(cb *payment.MidtransCallback) bool {
			return cb.MidtransOrderID == payload.OrderID
		})).
		Return(nil)

	// UpdateCallback dipanggil setelah signature dicek (SignatureValid = false)
	repo.EXPECT().
		UpdateCallback(context.Background(), mock.MatchedBy(func(cb *payment.MidtransCallback) bool {
			return !cb.SignatureValid
		})).
		Return(nil)

	err := svc.HandleMidtransWebhook(context.Background(), rawBody, payload)

	// Midtrans selalu harus dapat HTTP 200 — service tidak boleh return error
	require.NoError(t, err)
}

func TestHandleMidtransWebhook_ValidSignature_PaymentNotFound_StillReturnsNil(t *testing.T) {
	svc, repo := newSvc(t)

	orderID := "order-002"
	statusCode := "200"
	grossAmount := "25000.00"

	payload := payment.MidtransWebhookPayload{
		OrderID:      orderID,
		StatusCode:   statusCode,
		GrossAmount:  grossAmount,
		SignatureKey: midtransSignature(orderID, statusCode, grossAmount, testServerKey),
	}
	rawBody, _ := json.Marshal(payload)

	repo.EXPECT().
		LogCallback(context.Background(), mock.MatchedBy(func(cb *payment.MidtransCallback) bool {
			return cb.MidtransOrderID == orderID
		})).
		Return(nil)

	repo.EXPECT().
		UpdateCallback(context.Background(), mock.MatchedBy(func(cb *payment.MidtransCallback) bool {
			return cb.SignatureValid
		})).
		Return(nil)

	repo.EXPECT().
		FindPaymentByMidtransOrderID(context.Background(), orderID).
		Return(nil, assert.AnError)

	err := svc.HandleMidtransWebhook(context.Background(), rawBody, payload)

	require.NoError(t, err)
}

func TestHandleMidtransWebhook_SignatureKeyChangedByOneByte_IsRejected(t *testing.T) {
	svc, repo := newSvc(t)

	orderID := "order-003"
	statusCode := "200"
	grossAmount := "10000.00"
	validSig := midtransSignature(orderID, statusCode, grossAmount, testServerKey)

	// Flip last character of signature
	tampered := validSig[:len(validSig)-1] + "X"

	payload := payment.MidtransWebhookPayload{
		OrderID:      orderID,
		StatusCode:   statusCode,
		GrossAmount:  grossAmount,
		SignatureKey: tampered,
	}
	rawBody, _ := json.Marshal(payload)

	repo.EXPECT().
		LogCallback(context.Background(), mock.Anything).
		Return(nil)

	repo.EXPECT().
		UpdateCallback(context.Background(), mock.MatchedBy(func(cb *payment.MidtransCallback) bool {
			return !cb.SignatureValid
		})).
		Return(nil)

	err := svc.HandleMidtransWebhook(context.Background(), rawBody, payload)
	require.NoError(t, err)
}

func TestHandleMidtransWebhook_OrderIDChangedInPayload_SignatureInvalid(t *testing.T) {
	svc, repo := newSvc(t)

	orderID := "order-004"
	statusCode := "200"
	grossAmount := "5000.00"

	// Build signature for original orderID but send different orderID in payload
	sig := midtransSignature(orderID, statusCode, grossAmount, testServerKey)

	payload := payment.MidtransWebhookPayload{
		OrderID:      "order-TAMPERED",
		StatusCode:   statusCode,
		GrossAmount:  grossAmount,
		SignatureKey: sig,
	}
	rawBody, _ := json.Marshal(payload)

	repo.EXPECT().LogCallback(context.Background(), mock.Anything).Return(nil)
	repo.EXPECT().
		UpdateCallback(context.Background(), mock.MatchedBy(func(cb *payment.MidtransCallback) bool {
			return !cb.SignatureValid
		})).
		Return(nil)

	err := svc.HandleMidtransWebhook(context.Background(), rawBody, payload)
	require.NoError(t, err)
}
