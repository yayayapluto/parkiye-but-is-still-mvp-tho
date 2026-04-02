package notification

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"parkieee/internal/modules/auth"
	"parkieee/pkg/logger"
	"parkieee/pkg/middleware"
)

type Module struct {
	Svc ServicePort
}

func NewModule(db *gorm.DB, log logger.Logger) *Module {
	repo := NewRepository(db)
	svc := NewService(repo, log)
	return &Module{Svc: svc}
}

func (m *Module) RegisterRoutes(router fiber.Router, authSvc auth.ServicePort) {
	h := newHandler(m.Svc)

	// Stream endpoint needs special SSE auth (accepts token in query)
	router.Get("/notifications/stream", middleware.AuthSSE(authSvc), h.stream)

	// Other endpoints use standard auth
	group := router.Group("/notifications", middleware.Auth(authSvc))
	group.Get("/", h.listForUser)
	group.Get("/unread-count", h.getUnreadCount)
	group.Patch("/:id/read", h.markRead)
	group.Patch("/read-all", h.markAllRead)
}
