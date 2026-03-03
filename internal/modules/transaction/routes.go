package transaction

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/config"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator, s3cfg config.S3Config) {
	adapter := newHTTPAdapter(svc, v, s3cfg)
	authMw := middleware.Auth(auth)

	txs := router.Group("/transactions", authMw)

	// Reads — semua role authenticated.
	txs.Get("/", adapter.h.listTransactions)
	txs.Get("/code/:code", adapter.h.getByCode)
	txs.Get("/:id", adapter.h.getTransaction)
	txs.Get("/:id/logs", adapter.h.getLogs)

	// Writes — semua role authenticated bisa, karena operator adalah pelaksana utama.
	// Permission gate.override sudah cukup ketat di level override module.
	txs.Post("/entry", adapter.h.recordEntry)
	txs.Post("/:id/exit", adapter.h.recordExit)
	txs.Post("/:id/cancel", adapter.h.cancel)
}
