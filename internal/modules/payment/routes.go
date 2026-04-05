package payment

import (
	"github.com/gofiber/fiber/v2"
	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, gateSvc middleware.GateTokenValidator, assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort, v *validator.Validator, isSandbox bool) {
	adapter := newHTTPAdapter(svc, v, assignmentRepo)
	authMw := middleware.Auth(auth)
	gateAuthMw := middleware.GateAuth(gateSvc)
	authSseMw := middleware.AuthSSE(auth)
	gateAuthSseMw := middleware.GateAuthSSE(gateSvc)
	refundMw := middleware.RequirePermission("payment.refund")
	cashierMw := middleware.RequirePermission("cashier.ability")

	// Webhook di luar group auth — Midtrans tidak kirim JWT
	router.Post("/payments/webhook/midtrans", adapter.h.webhookMidtrans)

	// Kasir SSE — pakai authSseMw agar EventSource bisa kirim token via ?token=
	router.Get("/payments/cashier/listen", authSseMw, cashierMw, adapter.h.listenAsCashier)
	router.Get("/payments/cashier/pending", authMw, cashierMw, adapter.h.pendingCashierRequests)

	p := router.Group("/payments", authMw)
	p.Get("/", adapter.h.listPayments)
	p.Get("/transaction/:txID", adapter.h.listByTransaction)
	p.Get("/:id", adapter.h.getPayment)
	p.Post("/cash", adapter.h.payCash)
	p.Post("/qris", adapter.h.initiateQRIS)
	p.Get("/:id", adapter.h.getPayment)
	p.Post("/cashier/done", cashierMw, adapter.h.notifyCashierToKiosk)

	r := router.Group("/payments/refunds", authMw)
	r.Post("/", refundMw, adapter.h.requestRefund)
	r.Get("/", adapter.h.listRefunds)
	r.Patch("/:id/approve", refundMw, adapter.h.approveRefund)
	r.Patch("/:id/reject", refundMw, adapter.h.rejectRefund)

	// Kiosk SSE — HARUS di luar group gp karena EventSource tidak bisa kirim header
	router.Get("/gate/payments/cashier/listen/:txID", gateAuthSseMw, adapter.h.listenAsKiosk)
	router.Post("/gate/payments/cashier/notify", gateAuthMw, adapter.h.notifyKioskToCashier)
	router.Get("/gate/payments/cashier/status", gateAuthMw, adapter.h.cashierStatus)

	gp := router.Group("/gate/payments", gateAuthMw)
	gp.Post("/cash", adapter.h.payCash)
	gp.Post("/qris", adapter.h.initiateQRIS)
	gp.Get("/transaction/:txID", adapter.h.listByTransaction)
	gp.Get("/:id/poll", adapter.h.pollPaymentStatus)
	gp.Get("/:id", adapter.h.getPayment)

	if isSandbox {
		router.Get("/payments/qris-sim", adapter.h.qrisSimPage)
		router.Post("/payments/sim-pay", authMw, adapter.h.simulatePay)
	}
}
