package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/middleware"
)

// UserRepositoryPort handles all DB operations for the users table.
// Soft delete only — never hard delete a user.
type UserRepositoryPort interface {
	// FindByID returns a user with their role preloaded.
	// Returns ErrNotFound if the user doesn't exist or is soft-deleted.
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)

	// FindByEmail is mainly used during login. Also preloads role.
	// Returns ErrNotFound if no match — caller decides whether to expose that to client.
	FindByEmail(ctx context.Context, email string) (*User, error)

	// Create inserts a new user. Email uniqueness is enforced at DB level,
	// but service layer should check first for a cleaner error message.
	Create(ctx context.Context, user *User) error

	// Update persists changes to name, email, password hash, role, or is_active.
	// Does NOT update deleted_at — use SoftDelete for that.
	Update(ctx context.Context, user *User) error

	// SoftDelete sets deleted_at to now. User can no longer login after this.
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// RoleRepositoryPort handles roles. For now roles are mostly static
// (seeded at startup), but admin might want to view or add new ones.
type RoleRepositoryPort interface {
	// FindByID fetches a role with its permissions preloaded.
	FindByID(ctx context.Context, id uuid.UUID) (*Role, error)

	// FindByName looks up by role name string e.g. "admin", "operator".
	FindByName(ctx context.Context, name string) (*Role, error)

	// FindAll returns all roles. Used in admin UI for dropdowns etc.
	FindAll(ctx context.Context) ([]Role, error)

	// Create adds a new role. Not exposed in initial MVP routes,
	// but keeping it here so service layer isn't blocked later.
	Create(ctx context.Context, role *Role) error
}

// PermissionRepositoryPort handles the permissions table.
// Permissions are seeded and mostly read-only, but we still need CRUD
// for the admin management surface.
type PermissionRepositoryPort interface {
	// FindByID fetches a single permission by its UUID.
	FindByID(ctx context.Context, id uuid.UUID) (*Permission, error)

	// FindByNode looks up by node string e.g. "gate.override", "fee.edit".
	// Useful when checking if a node exists before assigning it.
	FindByNode(ctx context.Context, node string) (*Permission, error)

	// FindAll returns everything in the permissions table.
	FindAll(ctx context.Context) ([]Permission, error)
}

// RolePermissionRepositoryPort manages the many-to-many between roles and permissions.
type RolePermissionRepositoryPort interface {
	// FindByRoleID returns all permission nodes for a given role.
	// This is called when building JWT claims after login.
	FindByRoleID(ctx context.Context, roleID uuid.UUID) ([]RolePermission, error)

	// Assign links a permission to a role. Should be idempotent —
	// if the pair already exists, just return nil.
	Assign(ctx context.Context, roleID, permissionID uuid.UUID, grantedBy uuid.UUID) error

	// Revoke removes a permission from a role.
	// Returns ErrNotFound if the pair doesn't exist.
	Revoke(ctx context.Context, roleID, permissionID uuid.UUID) error

	// HasPermission is a quick existence check — used internally by service
	// to avoid hitting FindByRoleID + loop every time.
	HasPermission(ctx context.Context, roleID uuid.UUID, node string) (bool, error)
}

// SessionRepositoryPort manages user_sessions — one row per active login.
type SessionRepositoryPort interface {
	// Create inserts a new session after successful login.
	// TokenHash should already be hashed by service before calling this.
	Create(ctx context.Context, session *UserSession) error

	// FindByTokenHash looks up an active (not revoked, not expired) session.
	// Returns ErrNotFound if no match — middleware treats this as unauthorized.
	FindByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error)

	// Revoke sets revoked_at to now for a specific session (single logout).
	Revoke(ctx context.Context, id uuid.UUID) error

	// RevokeAllByUserID revokes every session for a user — useful when
	// password changes or admin force-logouts someone.
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error

	// DeleteExpired is a housekeeping method — removes sessions past their expiry.
	// Can be called from a cron or just before session creation to keep the table lean.
	DeleteExpired(ctx context.Context, before time.Time) error
}

// LoginLogRepositoryPort is append-only — we never update or delete login logs.
type LoginLogRepositoryPort interface {
	// Append records a login or logout attempt.
	// UserID is nullable because we log failed attempts for unknown emails too.
	Append(ctx context.Context, log *UserLoginLog) error
}

// LoginStatsRepositoryPort manages pre-computed counters for lockout checks.
// Upsert pattern — one row per user, created on first login attempt.
type LoginStatsRepositoryPort interface {
	// FindByUserID returns the stats row for a user.
	// Returns ErrNotFound if user has never attempted login.
	FindByUserID(ctx context.Context, userID uuid.UUID) (*UserLoginStats, error)

	// Upsert creates or updates the stats for a user.
	// Call this after every login attempt to keep counters accurate.
	Upsert(ctx context.Context, stats *UserLoginStats) error
}

// ServicePort is the public API of the auth module.
// Other modules and middleware should only depend on this interface, never on
// the concrete service struct or any repo directly.
type ServicePort interface {
	// Login validates credentials, creates a session, and returns a signed JWT.
	// Handles lockout checks and logs the attempt regardless of outcome.
	Login(ctx context.Context, email, password, ip, userAgent string) (*LoginResponse, error)

	// Logout revokes the session tied to the given token.
	Logout(ctx context.Context, token string) error

	// ValidateToken verifies the JWT signature and checks the session is still active.
	// Returns claims that middleware puts into fiber.Locals.
	ValidateToken(ctx context.Context, token string) (*middleware.TokenClaims, error)

	// GetProfile returns a user with their role, used for the /me endpoint.
	GetProfile(ctx context.Context, userID uuid.UUID) (*User, error)

	// CreateUser is admin-only. Creates a new user and assigns them a role.
	CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error)

	// UpdateUser can change name, email, or role. Password change is separate.
	UpdateUser(ctx context.Context, userID uuid.UUID, req *UpdateUserRequest) (*User, error)

	// ChangePassword validates the old password before setting the new one.
	// Also revokes all active sessions to force re-login everywhere.
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error

	// DeactivateUser soft-deletes the user and revokes all their sessions.
	// Cannot deactivate yourself or the last active admin.
	DeactivateUser(ctx context.Context, userID uuid.UUID, actorID uuid.UUID) error

	// GetAllRoles returns all available roles. Used in admin dropdowns.
	GetAllRoles(ctx context.Context) ([]Role, error)

	// GetAllPermissions returns all permission nodes with descriptions.
	GetAllPermissions(ctx context.Context) ([]Permission, error)

	// AssignPermission grants a permission node to a role.
	// actorID is the admin doing the action — logged to audit.
	AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID, actorID uuid.UUID) error

	// RevokePermission removes a permission from a role.
	RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID, actorID uuid.UUID) error

	// CheckPermission returns true if the user's role has the given permission node.
	// Mainly used by RequirePermission middleware.
	CheckPermission(ctx context.Context, userID uuid.UUID, node string) (bool, error)
}
