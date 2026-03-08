package transaction

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/config"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, gateAuth middleware.GateTokenValidator, v *validator.Validator, s3cfg config.S3Config, cfg config.Config) {
	h := newHandler(svc, v, s3cfg)
	authMw := middleware.Auth(auth)
	gateAuthMw := middleware.GateAuth(gateAuth)

	// ── Operator routes (JWT user) ─────────────────────────────────────────
	txs := router.Group("/transactions", authMw)
	txs.Get("/", h.listTransactions)
	txs.Get("/code/:code", h.getByCode)
	txs.Get("/:id", h.getTransaction)
	txs.Get("/:id/logs", h.getLogs)
	txs.Post("/:id/cancel", h.cancel)

	// ── Kiosk routes (JWT gate) ────────────────────────────────────────────
	// Gate-authenticated transaction endpoints.
	// Pakai prefix /gate/transactions agar tidak overlap dengan /kiosk group.
	gateTx := router.Group("/gate/transactions", gateAuthMw)
	gateTx.Get("/", h.listTransactions) // harus duluan sebelum /:id
	gateTx.Get("/code/:code", h.getByCode)
	gateTx.Get("/rfid/:uid", h.getOpenByRFID)
	gateTx.Get("/:id", h.getTransaction)
	gateTx.Post("/entry", h.recordEntry)
	gateTx.Post("/:id/exit", h.recordExit)
	// ONLY IN DEVELOPMENT
	if cfg.IsDevelopment() {
		gateTx.Patch("/:id/simulate", h.simulate)
	}
}
