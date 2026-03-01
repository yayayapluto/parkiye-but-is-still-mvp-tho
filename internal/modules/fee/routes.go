package fee

import (
	"github.com/gofiber/fiber/v2"

	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)

	feeGroup := router.Group("/fee", middleware.Auth(auth))

	configs := feeGroup.Group("/configs")
	configs.Get("/", adapter.h.listFeeConfigs)
	configs.Get("/:id", adapter.h.getFeeConfig)

	manage := feeGroup.Group("", middleware.RequirePermission("fee.edit"))
	manage.Post("/configs", adapter.h.createFeeConfig)
	manage.Delete("/configs/:id", adapter.h.deactivateFeeConfig)

	holidayRates := feeGroup.Group("/holiday-rates")
	holidayRates.Get("/", adapter.h.listHolidayRates)
	holidayRates.Get("/:id", adapter.h.getHolidayRate)

	holidayManage := feeGroup.Group("/holiday-rates", middleware.RequirePermission("fee.edit"))
	holidayManage.Post("/", adapter.h.createHolidayRate)
	holidayManage.Patch("/:id", adapter.h.updateHolidayRate)
	holidayManage.Delete("/:id", adapter.h.deleteHolidayRate)
}
