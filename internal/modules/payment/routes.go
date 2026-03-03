package payment

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)
	authMw := middleware.Auth(auth)
	managerMw := middleware.RequirePermission("config.edit")

	// Webhook harus di luar group authMw karena Midtrans tidak kirim JWT
	router.Post("/payments/webhook/midtrans", adapter.h.webhookMidtrans)

	p := router.Group("/payments", authMw)
	// route spesifik (/transaction/:txID) harus didaftarkan SEBELUM route parameter (/:id)
	p.Get("/transaction/:txID", adapter.h.listByTransaction)
	p.Get("/:id", adapter.h.getPayment)
	p.Post("/cash", adapter.h.payCash)
	p.Post("/qris", adapter.h.initiateQRIS)

	r := router.Group("/payments/refunds", authMw)
	r.Post("/", adapter.h.requestRefund)
	r.Get("/", adapter.h.listRefunds)
	r.Patch("/:id/approve", managerMw, adapter.h.approveRefund)
	r.Patch("/:id/reject", managerMw, adapter.h.rejectRefund)
}
