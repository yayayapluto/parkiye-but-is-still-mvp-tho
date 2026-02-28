package auth

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)

	auth := router.Group("/auth")
	auth.Post("/login", adapter.h.login)

	protected := auth.Group("", middleware.Auth(svc))
	protected.Post("/logout", adapter.h.logout)
	protected.Get("/me", adapter.h.me)
	protected.Patch("/change-password", adapter.h.changePassword)

	users := protected.Group("/users", middleware.RequireRole("admin", "owner"))
	users.Post("/", adapter.h.createUser)
	users.Put("/:id", adapter.h.updateUser)
	users.Delete("/:id", adapter.h.deactivateUser)

	roles := protected.Group("/roles", middleware.RequireRole("admin", "engineer"))
	roles.Get("/", adapter.h.getRoles)
	roles.Post("/:id/permissions", adapter.h.assignPermission)
	roles.Delete("/:id/permissions", adapter.h.revokePermission)

	perms := protected.Group("/permissions", middleware.RequireRole("admin", "engineer"))
	perms.Get("/", adapter.h.getPermissions)
}
