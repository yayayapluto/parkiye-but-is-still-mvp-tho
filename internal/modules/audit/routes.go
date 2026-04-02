package audit

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator) {
	adapter := newHTTPAdapter(svc)
	authMw := middleware.Auth(auth)
	auditReadMw := middleware.RequirePermission("audit.read")

	router.Get("/audit-logs", authMw, auditReadMw, adapter.h.listAuditLogs)
	router.Get("/audit-logs/:id", authMw, auditReadMw, adapter.h.getAuditLog)
}
