package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator) {
	h := newHandler(svc)
	authMw := middleware.Auth(auth)

	router.Get("/dashboard/stats", authMw, h.getStats)
	router.Get("/dashboard/user-role-summary", authMw, h.getUserRoleSummary)
}
