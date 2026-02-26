package middleware

import (
	"strings"

	"parkieee/internal/modules/auth"
	"parkieee/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func Auth(authService auth.ServicePort) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Error(c, fiber.StatusUnauthorized, "missing authorization header")
		}

		// Check Bearer scheme
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Error(c, fiber.StatusUnauthorized, "invalid authorization header")
		}

		// Validate token
		claims, err := authService.ValidateToken(c.Context(), parts[1])
		if err != nil {
			return response.Error(c, fiber.StatusUnauthorized, "invalid or expired token")
		}

		// Set user info in context
		c.Locals("user_id", claims.UserID)
		c.Locals("user_role", claims.Role)
		c.Locals("permissions", claims.Permissions)

		return c.Next()
	}
}

func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		perms, ok := c.Locals("permissions").([]string)
		if !ok {
			return response.Error(c, fiber.StatusForbidden, "access denied")
		}

		for _, p := range perms {
			if p == permission {
				return c.Next()
			}
		}

		return response.Error(c, fiber.StatusForbidden, "insufficient permissions")
	}
}

type TokenClaims struct {
	UserID      uuid.UUID
	Email       string
	Role        string
	Permissions []string
}