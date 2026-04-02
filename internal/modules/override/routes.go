package override

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, authSvc middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)
	g := router.Group("/overrides", middleware.Auth(authSvc))
	g.Get("/", adapter.h.listOverrides)
	g.Get("/:id", adapter.h.getOverride)
}
