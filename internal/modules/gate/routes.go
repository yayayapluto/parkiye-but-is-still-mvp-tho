package gate

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, authSvc middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)

	g := router.Group("/gate")

	// ── Token-first flow ──────────────────────────────────────────────────────
	// POST /api/v1/gate/authenticate
	// Screen kirim gate_token → dapat JWT long-lived.
	g.Post("/authenticate", adapter.h.authenticate)

	// ── QR Pairing flow ───────────────────────────────────────────────────────
	// POST /api/v1/gate/pairing/request
	// Screen request pairing code baru. Tidak butuh auth.
	g.Post("/pairing/request", adapter.h.requestPairing)

	// GET /api/v1/gate/pairing/:code/listen  (SSE)
	// Screen subscribe dan nunggu JWT. Tidak butuh auth.
	g.Get("/pairing/:code/listen", adapter.h.listenPairing)

	// Endpoint admin — butuh auth
	adminPairing := g.Group("/pairing")
	gatePairMw := middleware.RequirePermission("gate.pair")
	adminPairing.Use(middleware.Auth(authSvc), gatePairMw)

	// GET /api/v1/gate/pairing/:code
	// Admin lihat info pairing setelah scan QR.
	adminPairing.Get("/:code", adapter.h.getPairingInfo)

	// POST /api/v1/gate/pairing/:code/confirm
	// Admin assign screen ke gate tertentu.
	adminPairing.Post("/:code/confirm", adapter.h.confirmPairing)
}
