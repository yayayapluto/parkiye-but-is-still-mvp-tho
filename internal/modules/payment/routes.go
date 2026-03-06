package payment

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, gateSvc middleware.GateTokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)
	authMw := middleware.Auth(auth)
	gateAuthMw := middleware.GateAuth(gateSvc)
	managerMw := middleware.RequirePermission("config.edit")

	// Webhook harus di luar group authMw karena Midtrans tidak kirim JWT
	router.Post("/payments/webhook/midtrans", adapter.h.webhookMidtrans)

	p := router.Group("/payments", authMw)
	// route spesifik (/transaction/:txID) harus didaftarkan SEBELUM route parameter (/:id)
	p.Get("/transaction/:txID", adapter.h.listByTransaction)
	p.Get("/:id", adapter.h.getPayment)
	p.Post("/cash", adapter.h.payCash)
	p.Post("/qris", adapter.h.initiateQRIS)

	// Route payment untuk kiosk gate — memakai gate token, bukan user JWT
	gp := router.Group("/gate/payments", gateAuthMw)
	gp.Post("/cash", adapter.h.payCash)
	gp.Post("/qris", adapter.h.initiateQRIS)
	gp.Get("/transaction/:txID", adapter.h.listByTransaction)
	gp.Get("/:id/poll", adapter.h.pollPaymentStatus)
	gp.Get("/:id", adapter.h.getPayment)

	r := router.Group("/payments/refunds", authMw)
	r.Post("/", adapter.h.requestRefund)
	r.Get("/", adapter.h.listRefunds)
	r.Patch("/:id/approve", managerMw, adapter.h.approveRefund)
	r.Patch("/:id/reject", managerMw, adapter.h.rejectRefund)
}
