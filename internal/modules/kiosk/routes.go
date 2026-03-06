package kiosk

import (
	"parkieee/internal/modules/fee"
	"parkieee/internal/modules/vehicle"
	"parkieee/internal/modules/zone"
	"parkieee/pkg/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(
	router fiber.Router,
	feeSvc fee.ServicePort,
	vehicleSvc vehicle.ServicePort,
	zoneSvc zone.ServicePort,
	gateAuth middleware.GateTokenValidator,
) {
	h := newHandler(feeSvc, vehicleSvc, zoneSvc)

	g := router.Group("/kiosk", middleware.GateAuth(gateAuth))
	g.Get("/tariff", h.getTariff)
}
