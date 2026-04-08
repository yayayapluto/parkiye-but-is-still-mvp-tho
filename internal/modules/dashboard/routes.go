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
	router.Get("/dashboard/operator", authMw, h.getOperatorDashboard)
	router.Get("/dashboard/owner", authMw, h.getOwnerDashboard)
	router.Get("/dashboard/admin", authMw, h.getAdminDashboard)
	router.Get("/dashboard/engineer", authMw, h.getEngineerDashboard)
	router.Get("/dashboard/cashier", authMw, h.getCashierDashboard)
}
