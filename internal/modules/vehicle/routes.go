package vehicle

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)

	authMw := middleware.Auth(auth)
	manageMw := middleware.RequireRole("admin", "owner", "engineer")

	vt := router.Group("/vehicle-types", authMw)
	vt.Get("", adapter.h.listVehicleTypes)
	vt.Get("/:id", adapter.h.getVehicleType)
	vt.Post("", manageMw, adapter.h.createVehicleType)
	vt.Patch("/:id", manageMw, adapter.h.updateVehicleType)
	vt.Delete("/:id", manageMw, adapter.h.deleteVehicleType)

	veh := router.Group("/vehicles", authMw)
	veh.Get("", adapter.h.listVehicles)
	veh.Get("/plate/:plate", adapter.h.getVehicleByPlate)
	veh.Get("/:id", adapter.h.getVehicle)
	veh.Post("", adapter.h.upsertVehicle)
	veh.Patch("/:id", manageMw, adapter.h.updateVehicle)
}
