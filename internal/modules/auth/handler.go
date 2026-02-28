package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"parkieee/pkg/middleware"
	"parkieee/pkg/response"
	"parkieee/pkg/validator"
)

type handler struct {
	svc ServicePort
	v   *validator.Validator
}

func newHandler(svc ServicePort, v *validator.Validator) *handler {
	return &handler{svc: svc, v: v}
}

func (h *handler) login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	ip := c.IP()
	userAgent := c.Get("User-Agent")

	result, err := h.svc.Login(c.Context(), req.Email, req.Password, ip, userAgent)
	if err != nil {
		return err
	}

	return response.Success(c, "login successful", LoginResponse{
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
		User:      result.User,
	})
}

func (h *handler) logout(c *fiber.Ctx) error {
	token := extractToken(c)
	if token == "" {
		return response.BadRequest(c, "missing token", nil)
	}

	if err := h.svc.Logout(c.Context(), token); err != nil {
		return err
	}

	return response.Success(c, "logout successful", nil)
}

func (h *handler) me(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)

	user, err := h.svc.GetProfile(c.Context(), userID)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", toUserResponse(user))
}

func (h *handler) changePassword(c *fiber.Ctx) error {
	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	userID := c.Locals("user_id").(uuid.UUID)

	if err := h.svc.ChangePassword(c.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		return err
	}

	return response.Success(c, "password changed successfully", nil)
}

func (h *handler) createUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	// CreateUserRequest di dto.go dan ports.go berbeda struct —
	// map manual di sini supaya service tidak import dto.
	svcReq := &CreateUserRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		RoleID:   req.RoleID,
	}

	user, err := h.svc.CreateUser(c.Context(), svcReq)
	if err != nil {
		return err
	}

	return response.Created(c, "user created", toUserResponse(user))
}

func (h *handler) updateUser(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid user id", nil)
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	svcReq := &UpdateUserRequest{
		Name:   req.Name,
		Email:  req.Email,
		RoleID: req.RoleID,
	}

	user, err := h.svc.UpdateUser(c.Context(), id, svcReq)
	if err != nil {
		return err
	}

	return response.Success(c, "user updated", toUserResponse(user))
}

func (h *handler) deactivateUser(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid user id", nil)
	}

	actorID := c.Locals("user_id").(uuid.UUID)

	if err := h.svc.DeactivateUser(c.Context(), id, actorID); err != nil {
		return err
	}

	return response.Success(c, "user deactivated", nil)
}

func (h *handler) getRoles(c *fiber.Ctx) error {
	roles, err := h.svc.GetAllRoles(c.Context())
	if err != nil {
		return err
	}

	res := make([]RoleResponse, 0, len(roles))
	for i := range roles {
		res = append(res, toRoleResponse(&roles[i]))
	}

	return response.Success(c, "ok", res)
}

func (h *handler) getPermissions(c *fiber.Ctx) error {
	perms, err := h.svc.GetAllPermissions(c.Context())
	if err != nil {
		return err
	}

	res := make([]PermissionResponse, 0, len(perms))
	for i := range perms {
		res = append(res, toPermissionResponse(&perms[i]))
	}

	return response.Success(c, "ok", res)
}

func (h *handler) assignPermission(c *fiber.Ctx) error {
	roleID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid role id", nil)
	}

	var req AssignPermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	actorID := c.Locals("user_id").(uuid.UUID)

	if err := h.svc.AssignPermission(c.Context(), roleID, req.PermissionID, actorID); err != nil {
		return err
	}

	return response.Success(c, "permission assigned", nil)
}

func (h *handler) revokePermission(c *fiber.Ctx) error {
	roleID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid role id", nil)
	}

	var req AssignPermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	actorID := c.Locals("user_id").(uuid.UUID)

	if err := h.svc.RevokePermission(c.Context(), roleID, req.PermissionID, actorID); err != nil {
		return err
	}

	return response.Success(c, "permission revoked", nil)
}

// extractToken ambil raw JWT dari Authorization header.
func extractToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

// mustUserID helper untuk ambil user_id dari locals — panic kalau tidak ada,
// artinya route tidak dipasangi Auth middleware.
func mustUserID(c *fiber.Ctx) uuid.UUID {
	return c.Locals("user_id").(uuid.UUID)
}

// currentClaims helper untuk ambil full claims dari locals.
func currentClaims(c *fiber.Ctx) *middleware.TokenClaims {
	claims, _ := c.Locals("claims").(*middleware.TokenClaims)
	return claims
}
