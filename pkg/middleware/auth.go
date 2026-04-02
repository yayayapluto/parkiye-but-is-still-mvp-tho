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

// AuthSSE adalah Auth yang juga menerima token via ?token= query param.
// Dipakai untuk SSE endpoint karena EventSource browser tidak support custom header.
func AuthSSE(svc TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Query("token")
		if token == "" {
			authHeader := c.Get("Authorization")
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
		if token == "" {
			return response.Unauthorized(c, "Token tidak ditemukan")
		}
		claims, err := svc.ValidateToken(c.Context(), token)
		if err != nil {
			return response.Unauthorized(c, "Token tidak valid atau sudah kedaluwarsa")
		}
		c.Locals("user_id", claims.UserID)
		c.Locals("user_role", claims.Role)
		c.Locals("permissions", claims.Permissions)
		c.Locals("user_email", claims.Email)
		c.Locals("claims", claims)
		return c.Next()
	}
}

// GateAuthSSE adalah GateAuth yang juga menerima token via ?token= query param.
func GateAuthSSE(svc GateTokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Query("token")
		if token == "" {
			authHeader := c.Get("Authorization")
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
		if token == "" {
			return response.Unauthorized(c, "Token tidak ditemukan")
		}
		claims, err := svc.ValidateGateToken(c.Context(), token)
		if err != nil {
			return response.Unauthorized(c, "Token gate tidak valid atau sudah kedaluwarsa")
		}
		c.Locals("gate_id", claims.GateID)
		c.Locals("gate_type", claims.GateType)
		c.Locals("zone_id", claims.ZoneID)
		c.Locals("gate_name", claims.GateName)
		c.Locals("gate_claims", claims)
		return c.Next()
	}
}
func Auth(svc TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Preflight OPTIONS harus lolos tanpa auth supaya CORS header bisa dikirim
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}

		token := ""
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
		if token == "" {
			token = c.Cookies("access_token")
		}
		if token == "" {
			return response.Unauthorized(c, "Header otorisasi tidak ditemukan")
		}

		claims, err := svc.ValidateToken(c.Context(), token)
		if err != nil {
			return response.Unauthorized(c, "Token tidak valid atau sudah kedaluwarsa")
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
			return response.Forbidden(c, "Akses ditolak: izin tidak ditemukan")
		}
		for _, p := range perms {
			if p == permission {
				return c.Next()
			}
		}
		return response.Forbidden(c, "Izin tidak mencukupi: membutuhkan '"+permission+"'")
	}
}

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("user_role").(string)
		if !ok {
			return response.Forbidden(c, "Akses ditolak: role tidak ditemukan")
		}
		for _, role := range roles {
			if userRole == role {
				return c.Next()
			}
		}
		return response.Forbidden(c, "Akses ditolak: membutuhkan salah satu role: "+strings.Join(roles, ", "))
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
		// Preflight OPTIONS harus lolos tanpa auth supaya CORS header bisa dikirim
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Header otorisasi tidak ditemukan")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Format header otorisasi tidak valid")
		}

		claims, err := svc.ValidateGateToken(c.Context(), parts[1])
		if err != nil {
			return response.Unauthorized(c, "Token gate tidak valid atau sudah kedaluwarsa")
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
