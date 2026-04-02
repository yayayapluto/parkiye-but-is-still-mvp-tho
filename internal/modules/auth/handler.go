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
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	ip := c.IP()
	userAgent := c.Get("User-Agent")

	result, err := h.svc.Login(c.Context(), req.Identifier, req.Password, ip, userAgent, req.Remember)
	if err != nil {
		return err
	}

	isSecure := c.Protocol() == "https"

	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    result.Token,
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
		Expires:  result.ExpiresAt,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
		Expires:  result.RefreshExpiresAt,
		Path:     "/api/v1/auth/refresh",
	})

	return response.Success(c, "login successful", result)
}

func (h *handler) logout(c *fiber.Ctx) error {
	token := extractToken(c)
	if token == "" {
		return response.BadRequest(c, "Token tidak ditemukan", nil)
	}

	if err := h.svc.Logout(c.Context(), token); err != nil {
		return err
	}

	c.ClearCookie("access_token")
	c.ClearCookie("refresh_token")

	return response.Success(c, "logout successful", nil)
}

func (h *handler) refresh(c *fiber.Ctx) error {
	rawRefresh := c.Cookies("refresh_token")
	if rawRefresh == "" {
		return response.Unauthorized(c, "Refresh token tidak ditemukan")
	}

	ip := c.IP()
	userAgent := c.Get("User-Agent")

	result, err := h.svc.Refresh(c.Context(), rawRefresh, ip, userAgent)
	if err != nil {
		return err
	}

	isSecure := c.Protocol() == "https"

	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    result.Token,
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
		Expires:  result.ExpiresAt,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
		Expires:  result.RefreshExpiresAt,
		Path:     "/api/v1/auth/refresh",
	})

	return response.Success(c, "token refreshed", result)
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
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	svcReq := &CreateUserRequest{
		Name:     req.Name,
		Username: req.Username,
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
		return response.BadRequest(c, "ID pengguna tidak valid", nil)
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	svcReq := &UpdateUserRequest{
		Name:     req.Name,
		Username: req.Username,
		Email:    req.Email,
		RoleID:   req.RoleID,
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
		return response.BadRequest(c, "ID pengguna tidak valid", nil)
	}

	actorID := c.Locals("user_id").(uuid.UUID)

	if err := h.svc.DeactivateUser(c.Context(), id, actorID); err != nil {
		return err
	}

	return response.Success(c, "user deactivated", nil)
}

func (h *handler) listUsers(c *fiber.Ctx) error {
	pag := response.ParsePaginationRequest(c)

	var roleID *uuid.UUID
	if raw := c.Query("role_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return response.BadRequest(c, "ID role tidak valid", nil)
		}
		roleID = &id
	}

	var activeOnly *bool
	if raw := c.Query("is_active"); raw != "" {
		v := raw == "true"
		activeOnly = &v
	}

	users, total, err := h.svc.ListUsers(c.Context(), roleID, activeOnly, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	res := make([]UserResponse, 0, len(users))
	for i := range users {
		res = append(res, toUserResponse(&users[i]))
	}

	queryParams := map[string]string{}
	if roleID != nil {
		queryParams["role_id"] = roleID.String()
	}
	if activeOnly != nil {
		if *activeOnly {
			queryParams["is_active"] = "true"
		} else {
			queryParams["is_active"] = "false"
		}
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/auth/users",
		pag.Page, pag.PageSize, total,
		queryParams,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getUser(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID pengguna tidak valid", nil)
	}
	user, err := h.svc.GetUser(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toUserResponse(user))
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
		return response.BadRequest(c, "ID role tidak valid", nil)
	}

	var req AssignPermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		return response.BadRequest(c, "ID role tidak valid", nil)
	}

	var req AssignPermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	actorID := c.Locals("user_id").(uuid.UUID)

	if err := h.svc.RevokePermission(c.Context(), roleID, req.PermissionID, actorID); err != nil {
		return err
	}

	return response.Success(c, "permission revoked", nil)
}

// extractToken ambil raw JWT dari Authorization header, fallback ke cookie.
func extractToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	return c.Cookies("access_token")
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
