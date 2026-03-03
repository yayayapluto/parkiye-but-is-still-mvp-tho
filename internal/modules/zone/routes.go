package zone

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)

	authMw := middleware.Auth(auth)
	manageMw := middleware.RequireRole("admin", "owner", "engineer")

	router.Get("/zones", authMw, adapter.h.listZones)
	router.Get("/zones/:id", authMw, adapter.h.getZone)
	router.Get("/zones/:id/capacity", authMw, adapter.h.getCapacity)
	router.Get("/zones/:id/gates", authMw, adapter.h.listGates)
	router.Get("/zones/:id/gates/:gateId", authMw, adapter.h.getGate)

	router.Post("/zones", authMw, manageMw, adapter.h.createZone)
	router.Patch("/zones/:id", authMw, manageMw, adapter.h.updateZone)
	router.Delete("/zones/:id", authMw, manageMw, adapter.h.deactivateZone)
	router.Post("/zones/:id/gates", authMw, manageMw, adapter.h.createGate)
	router.Patch("/zones/:id/gates/:gateId", authMw, manageMw, adapter.h.updateGate)
	router.Delete("/zones/:id/gates/:gateId", authMw, manageMw, adapter.h.deactivateGate)
	router.Post("/zones/:id/gates/:gateId/regenerate-token", authMw, manageMw, adapter.h.regenerateGateToken)
}
