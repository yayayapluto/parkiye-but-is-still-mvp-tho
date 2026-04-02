package auth

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

// User represents an operator/admin/owner/engineer in the system.
type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name         string     `gorm:"type:varchar(100);not null"`
	Username     string     `gorm:"type:varchar(50);not null;default:''"`
	Email        string     `gorm:"type:varchar(150);uniqueIndex;not null"`
	PasswordHash string     `gorm:"type:text;not null"`
	RoleID       uuid.UUID  `gorm:"type:uuid;not null"`
	IsActive     bool       `gorm:"not null;default:true"`
	CreatedBy    *uuid.UUID `gorm:"type:uuid"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"index"`

	Role          *Role `gorm:"foreignKey:RoleID"`
	CreatedByUser *User `gorm:"foreignKey:CreatedBy"`
}

func (User) TableName() string { return "users" }

// Role defines a named set of permissions.
type Role struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string     `gorm:"type:varchar(50);uniqueIndex;not null"` // "operator"|"admin"|"owner"|"engineer"
	Description string     `gorm:"type:text"`
	CreatedBy   *uuid.UUID `gorm:"type:uuid"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`

	Permissions []RolePermission `gorm:"foreignKey:RoleID"`
}

func (Role) TableName() string { return "roles" }

// Permission is a named action node that can be granted to roles.
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Node        string    `gorm:"type:varchar(100);uniqueIndex;not null"` // e.g. "gate.override"
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (Permission) TableName() string { return "permissions" }

// RolePermission is the join table between roles and permissions.
type RolePermission struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	RoleID       uuid.UUID  `gorm:"type:uuid;not null;index"`
	PermissionID uuid.UUID  `gorm:"type:uuid;not null;index"`
	GrantedBy    *uuid.UUID `gorm:"type:uuid"`
	GrantedAt    time.Time  `gorm:"not null;default:now()"`

	Role       *Role       `gorm:"foreignKey:RoleID"`
	Permission *Permission `gorm:"foreignKey:PermissionID"`
}

func (RolePermission) TableName() string { return "role_permissions" }

// UserSession tracks one active session per login.
type UserSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	TokenHash string    `gorm:"type:text;not null"`
	IPAddress string    `gorm:"type:varchar(45)"`
	UserAgent string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time

	User *User `gorm:"foreignKey:UserID"`
}

func (UserSession) TableName() string { return "user_sessions" }

// UserLoginLog is append-only; every login/logout attempt is recorded.
type UserLoginLog struct {
	ID            uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID        *uuid.UUID             `gorm:"type:uuid;index"`
	IPAddress     string                 `gorm:"type:varchar(45)"`
	UserAgent     string                 `gorm:"type:text"`
	AttemptType   types.LoginAttemptType `gorm:"type:varchar(10);not null"` // "login"|"logout"
	Success       bool                   `gorm:"not null"`
	FailureReason *types.FailureReason   `gorm:"type:varchar(100)"`
	AttemptedAt   time.Time              `gorm:"not null;default:now()"`

	User *User `gorm:"foreignKey:UserID"`
}

func (UserLoginLog) TableName() string { return "user_login_logs" }

// RefreshToken menyimpan satu refresh token per sesi.
// TokenHash adalah SHA-256 dari raw token — token asli tidak pernah disimpan.
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index"`
	SessionID uuid.UUID  `gorm:"type:uuid;not null;index"` // relasi ke user_sessions
	TokenHash string     `gorm:"type:text;not null;uniqueIndex"`
	IPAddress string     `gorm:"type:varchar(45)"`
	UserAgent string     `gorm:"type:text"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	ExpiresAt time.Time  `gorm:"not null"`
	RevokedAt *time.Time
	ReplacedBy *uuid.UUID `gorm:"type:uuid"` // ID dari refresh token baru setelah rotation

	User    *User        `gorm:"foreignKey:UserID"`
	Session *UserSession `gorm:"foreignKey:SessionID"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// UserLoginStats holds pre-computed counters for fast lockout checks.
type UserLoginStats struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID              uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	TotalAttempts       int       `gorm:"not null;default:0"`
	TotalFailedAttempts int       `gorm:"not null;default:0"`
	LastAttemptAt       *time.Time
	LastSuccessAt       *time.Time
	LastFailedIP        *string `gorm:"type:varchar(45)"`
	IsLocked            bool    `gorm:"not null;default:false"`
	LockedAt            *time.Time
	LockedReason        *types.LockedReason `gorm:"type:varchar(100)"`
	UpdatedAt           time.Time           `gorm:"autoUpdateTime"`

	User *User `gorm:"foreignKey:UserID"`
}

func (UserLoginStats) TableName() string { return "user_login_stats" }
