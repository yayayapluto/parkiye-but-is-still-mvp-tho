package auth

import (
	"time"

	"github.com/google/uuid"
)

type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type CreateUserRequest struct {
	Name     string    `json:"name"     validate:"required,min=2,max=100"`
	Email    string    `json:"email"    validate:"required,email"`
	Password string    `json:"password" validate:"required,min=8"`
	RoleID   uuid.UUID `json:"role_id"  validate:"required"`
}

type UpdateUserRequest struct {
	Name   *string    `json:"name"    validate:"omitempty,min=2,max=100"`
	Email  *string    `json:"email"   validate:"omitempty,email"`
	RoleID *uuid.UUID `json:"role_id" validate:"omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// AssignPermissionRequest dipakai untuk assign maupun revoke — endpoint-nya
// yang membedakan aksi mana yang dijalankan.
type AssignPermissionRequest struct {
	PermissionID uuid.UUID `json:"permission_id" validate:"required"`
}

// LoginResponse adalah yang dikembalikan setelah login sukses.
// Token di sini adalah raw JWT — client simpan di header untuk request berikutnya.
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

type UserResponse struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Email     string       `json:"email"`
	IsActive  bool         `json:"is_active"`
	Role      RoleResponse `json:"role"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type RoleResponse struct {
	ID          uuid.UUID            `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
}

type PermissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Node        string    `json:"node"`
	Description string    `json:"description"`
}

// toUserResponse konversi dari domain ke response DTO.
// Dipanggil dari handler, bukan dari service — service return domain model.
func toUserResponse(u *User) UserResponse {
	role := RoleResponse{
		ID:          u.Role.ID,
		Name:        u.Role.Name,
		Description: u.Role.Description,
	}

	for _, rp := range u.Role.Permissions {
		if rp.Permission == nil {
			continue
		}
		role.Permissions = append(role.Permissions, PermissionResponse{
			ID:          rp.Permission.ID,
			Node:        rp.Permission.Node,
			Description: rp.Permission.Description,
		})
	}

	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		IsActive:  u.IsActive,
		Role:      role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func toRoleResponse(r *Role) RoleResponse {
	res := RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
	}

	for _, rp := range r.Permissions {
		if rp.Permission == nil {
			continue
		}
		res.Permissions = append(res.Permissions, PermissionResponse{
			ID:          rp.Permission.ID,
			Node:        rp.Permission.Node,
			Description: rp.Permission.Description,
		})
	}

	return res
}

func toPermissionResponse(p *Permission) PermissionResponse {
	return PermissionResponse{
		ID:          p.ID,
		Node:        p.Node,
		Description: p.Description,
	}
}
