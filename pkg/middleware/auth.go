package middleware

import (
	"strings"

	"parkieee/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TokenClaims struct {
	UserID      uuid.UUID
	Email       string
	Role        string
	Permissions []string
}

//func Auth(authService auth.ServicePort) fiber.Handler {
//	return func(c *fiber.Ctx) error {
//		// Get token from header
//		authHeader := c.Get("Authorization")
//		if authHeader == "" {
//			return response.Unauthorized(c, "missing authorization header")
//		}
//
//		// Check Bearer scheme
//		parts := strings.Split(authHeader, " ")
//		if len(parts) != 2 || parts[0] != "Bearer" {
//			return response.Unauthorized(c, "invalid authorization header format")
//		}
//
//		// Validate token
//		claims, err := authService.ValidateToken(c.Context(), parts[1])
//		if err != nil {
//			return response.Unauthorized(c, "invalid or expired token")
//		}
//
//		// Set user info in context
//		c.Locals("user_id", claims.UserID)
//		c.Locals("user_role", claims.Role)
//		c.Locals("permissions", claims.Permissions)
//		c.Locals("user_email", claims.Email)
//
//		return c.Next()
//	}
//}
//
//func RequirePermission(permission string) fiber.Handler {
//	return func(c *fiber.Ctx) error {
//		// Get permissions from context
//		perms, ok := c.Locals("permissions").([]string)
//		if !ok {
//			return response.Forbidden(c, "access denied: no permissions found")
//		}
//
//		// Check if user has required permission
//		for _, p := range perms {
//			if p == permission {
//				return c.Next()
//			}
//		}
//
//		// Get user info for better error message
//		userID := c.Locals("user_id")
//		role := c.Locals("user_role")
//
//		return response.Forbidden(c,
//			"insufficient permissions: required '"+permission+"'")
//	}
//}

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("user_role").(string)
		if !ok {
			return response.Forbidden(c, "access denied: no role found")
		}

		for _, role := range roles {
			if userRole == role {
				return c.Next()
			}
		}

		return response.Forbidden(c,
			"access denied: requires one of roles "+strings.Join(roles, ", "))
	}
}

//func GetUserID(c *fiber.Ctx) uuid.UUID {
//	userID, ok := c.Locals("user_id").(uuid.UUID)
//	if !ok {
//		return uuid.Nil
//	}
//	return userID
//}

func GetUserRole(c *fiber.Ctx) string {
	role, ok := c.Locals("user_role").(string)
	if !ok {
		return ""
	}
	return role
}

func GetUserEmail(c *fiber.Ctx) string {
	email, ok := c.Locals("user_email").(string)
	if !ok {
		return ""
	}
	return email
}

func GetPermissions(c *fiber.Ctx) []string {
	perms, ok := c.Locals("permissions").([]string)
	if !ok {
		return []string{}
	}
	return perms
}

// Optional middleware for optional authentication (doesn't error if no token)
//func OptionalAuth(authService auth.ServicePort) fiber.Handler {
//	return func(c *fiber.Ctx) error {
//		authHeader := c.Get("Authorization")
//		if authHeader == "" {
//			return c.Next()
//		}
//
//		parts := strings.Split(authHeader, " ")
//		if len(parts) != 2 || parts[0] != "Bearer" {
//			return c.Next()
//		}
//
//		claims, err := authService.ValidateToken(c.Context(), parts[1])
//		if err == nil {
//			c.Locals("user_id", claims.UserID)
//			c.Locals("user_role", claims.Role)
//			c.Locals("permissions", claims.Permissions)
//			c.Locals("user_email", claims.Email)
//		}
//
//		return c.Next()
//	}
//}
