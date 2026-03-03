package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"parkieee/pkg/response"
)

type TokenClaims struct {
	UserID      uuid.UUID
	Email       string
	Role        string
	Permissions []string
}

type GateClaims struct {
	GateID   uuid.UUID
	GateType string
	ZoneID   uuid.UUID
	GateName string
}

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
}

type TokenValidatorFunc func(ctx context.Context, token string) (*TokenClaims, error)

func (f TokenValidatorFunc) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	return f(ctx, token)
}

func Auth(svc TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "missing authorization header")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "invalid authorization header format")
		}

		claims, err := svc.ValidateToken(c.Context(), parts[1])
		if err != nil {
			return response.Unauthorized(c, "invalid or expired token")
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("user_role", claims.Role)
		c.Locals("permissions", claims.Permissions)
		c.Locals("user_email", claims.Email)
		c.Locals("claims", claims)

		return c.Next()
	}
}

func OptionalAuth(svc TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Next()
		}

		if claims, err := svc.ValidateToken(c.Context(), parts[1]); err == nil {
			c.Locals("user_id", claims.UserID)
			c.Locals("user_role", claims.Role)
			c.Locals("permissions", claims.Permissions)
			c.Locals("user_email", claims.Email)
			c.Locals("claims", claims)
		}

		return c.Next()
	}
}

func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		perms, ok := c.Locals("permissions").([]string)
		if !ok {
			return response.Forbidden(c, "access denied: no permissions found")
		}
		for _, p := range perms {
			if p == permission {
				return c.Next()
			}
		}
		return response.Forbidden(c, "insufficient permissions: required '"+permission+"'")
	}
}

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
		return response.Forbidden(c, "access denied: requires one of roles "+strings.Join(roles, ", "))
	}
}

func GetUserID(c *fiber.Ctx) uuid.UUID {
	id, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

func GetUserRole(c *fiber.Ctx) string {
	role, _ := c.Locals("user_role").(string)
	return role
}

func GetUserEmail(c *fiber.Ctx) string {
	email, _ := c.Locals("user_email").(string)
	return email
}

func GetPermissions(c *fiber.Ctx) []string {
	perms, ok := c.Locals("permissions").([]string)
	if !ok {
		return []string{}
	}
	return perms
}

func GetClaims(c *fiber.Ctx) *TokenClaims {
	claims, _ := c.Locals("claims").(*TokenClaims)
	return claims
}

// GateTokenValidator adalah interface yang diimplementasi oleh gate.ServicePort.
type GateTokenValidator interface {
	ValidateGateToken(ctx context.Context, token string) (*GateClaims, error)
}

// GateAuth middleware untuk endpoint yang hanya boleh diakses dari screen gate.
func GateAuth(svc GateTokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "missing authorization header")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "invalid authorization header format")
		}

		claims, err := svc.ValidateGateToken(c.Context(), parts[1])
		if err != nil {
			return response.Unauthorized(c, "invalid or expired gate token")
		}

		c.Locals("gate_id", claims.GateID)
		c.Locals("gate_type", claims.GateType)
		c.Locals("zone_id", claims.ZoneID)
		c.Locals("gate_name", claims.GateName)
		c.Locals("gate_claims", claims)

		return c.Next()
	}
}

func GetGateID(c *fiber.Ctx) uuid.UUID {
	id, ok := c.Locals("gate_id").(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

func GetGateType(c *fiber.Ctx) string {
	t, _ := c.Locals("gate_type").(string)
	return t
}

func GetZoneID(c *fiber.Ctx) uuid.UUID {
	id, ok := c.Locals("zone_id").(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

func GetGateClaims(c *fiber.Ctx) *GateClaims {
	claims, _ := c.Locals("gate_claims").(*GateClaims)
	return claims
}
