package transaction

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/config"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, gateAuth middleware.GateTokenValidator, v *validator.Validator, s3cfg config.S3Config) {
	adapter := newHTTPAdapter(svc, v, s3cfg)
	authMw := middleware.Auth(auth)
	gateAuthMw := middleware.GateAuth(gateAuth)

	// ── Operator routes (JWT user) ─────────────────────────────────────────
	txs := router.Group("/transactions", authMw)
	txs.Get("/", adapter.h.listTransactions)
	txs.Get("/code/:code", adapter.h.getByCode)
	txs.Get("/:id", adapter.h.getTransaction)
	txs.Get("/:id/logs", adapter.h.getLogs)
	txs.Post("/:id/cancel", adapter.h.cancel)

	// ── Kiosk routes (JWT gate) ────────────────────────────────────────────
	// Gate-authenticated transaction endpoints.
	// Pakai prefix /gate/transactions agar tidak overlap dengan /kiosk group.
	gateTx := router.Group("/gate/transactions", gateAuthMw)
	gateTx.Get("/", adapter.h.listTransactions) // harus duluan sebelum /:id
	gateTx.Get("/code/:code", adapter.h.getByCode)
	gateTx.Get("/rfid/:uid", adapter.h.getOpenByRFID)
	gateTx.Get("/:id", adapter.h.getTransaction)
	gateTx.Post("/entry", adapter.h.recordEntry)
	gateTx.Post("/:id/exit", adapter.h.recordExit)
	gateTx.Patch("/:id/simulate", adapter.h.simulate)
}
