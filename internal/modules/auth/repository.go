package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"parkieee/pkg/errors"
)

// userRepository implements UserRepositoryPort.
type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepositoryPort {
	return &userRepository{db: db}
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("Role.Permissions.Permission").
		Where("deleted_at IS NULL").
		First(&user, "id = ?", id).Error
	if err != nil {
		return nil, errors.FromDB(err, "user not found")
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("Role.Permissions.Permission").
		Where("deleted_at IS NULL").
		First(&user, "email = ?", email).Error
	if err != nil {
		return nil, errors.FromDB(err, "user not found")
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(user).Error, "")
}

func (r *userRepository) Update(ctx context.Context, user *User) error {
	// Select("*") + Updates agar zero-value fields (e.g. is_active=false) ikut tersimpan,
	// tanpa pakai Save yang dianggap ambiguous di GORM v1.30+.
	return errors.FromDB(
		r.db.WithContext(ctx).Model(user).Select("*").Updates(user).Error,
		"",
	)
}

func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", id).
		Update("deleted_at", now).Error
	return errors.FromDB(err, "user not found")
}

// roleRepository implements RoleRepositoryPort.
type roleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) RoleRepositoryPort {
	return &roleRepository{db: db}
}

func (r *roleRepository) FindByID(ctx context.Context, id uuid.UUID) (*Role, error) {
	var role Role
	err := r.db.WithContext(ctx).
		Preload("Permissions.Permission").
		First(&role, "id = ?", id).Error
	if err != nil {
		return nil, errors.FromDB(err, "role not found")
	}
	return &role, nil
}

func (r *roleRepository) FindByName(ctx context.Context, name string) (*Role, error) {
	var role Role
	err := r.db.WithContext(ctx).
		Preload("Permissions.Permission").
		First(&role, "name = ?", name).Error
	if err != nil {
		return nil, errors.FromDB(err, "role not found")
	}
	return &role, nil
}

func (r *roleRepository) FindAll(ctx context.Context) ([]Role, error) {
	var roles []Role
	err := r.db.WithContext(ctx).
		Preload("Permissions.Permission").
		Find(&roles).Error
	return roles, errors.FromDB(err, "")
}

func (r *roleRepository) Create(ctx context.Context, role *Role) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(role).Error, "")
}

// permissionRepository implements PermissionRepositoryPort.
type permissionRepository struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) PermissionRepositoryPort {
	return &permissionRepository{db: db}
}

func (r *permissionRepository) FindByID(ctx context.Context, id uuid.UUID) (*Permission, error) {
	var perm Permission
	err := r.db.WithContext(ctx).First(&perm, "id = ?", id).Error
	if err != nil {
		return nil, errors.FromDB(err, "permission not found")
	}
	return &perm, nil
}

func (r *permissionRepository) FindByNode(ctx context.Context, node string) (*Permission, error) {
	var perm Permission
	err := r.db.WithContext(ctx).First(&perm, "node = ?", node).Error
	if err != nil {
		return nil, errors.FromDB(err, "permission not found")
	}
	return &perm, nil
}

func (r *permissionRepository) FindAll(ctx context.Context) ([]Permission, error) {
	var perms []Permission
	err := r.db.WithContext(ctx).Find(&perms).Error
	return perms, errors.FromDB(err, "")
}

// rolePermissionRepository implements RolePermissionRepositoryPort.
type rolePermissionRepository struct {
	db *gorm.DB
}

func NewRolePermissionRepository(db *gorm.DB) RolePermissionRepositoryPort {
	return &rolePermissionRepository{db: db}
}

func (r *rolePermissionRepository) FindByRoleID(ctx context.Context, roleID uuid.UUID) ([]RolePermission, error) {
	var rps []RolePermission
	err := r.db.WithContext(ctx).
		Preload("Permission").
		Where("role_id = ?", roleID).
		Find(&rps).Error
	return rps, errors.FromDB(err, "")
}

func (r *rolePermissionRepository) Assign(ctx context.Context, roleID, permissionID uuid.UUID, grantedBy uuid.UUID) error {
	rp := RolePermission{
		ID:           uuid.New(),
		RoleID:       roleID,
		PermissionID: permissionID,
		GrantedBy:    &grantedBy,
		GrantedAt:    time.Now(),
	}
	// OnConflict DoNothing supaya idempotent — kalau pair sudah ada, skip tanpa error.
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rp).Error
	return errors.FromDB(err, "")
}

func (r *rolePermissionRepository) Revoke(ctx context.Context, roleID, permissionID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Delete(&RolePermission{})
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "permission assignment not found")
	}
	return nil
}

func (r *rolePermissionRepository) HasPermission(ctx context.Context, roleID uuid.UUID, node string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&RolePermission{}).
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ? AND permissions.node = ?", roleID, node).
		Count(&count).Error
	if err != nil {
		return false, errors.FromDB(err, "")
	}
	return count > 0, nil
}

// sessionRepository implements SessionRepositoryPort.
type sessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) SessionRepositoryPort {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, session *UserSession) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(session).Error, "")
}

func (r *sessionRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	var session UserSession
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&session).Error
	if err != nil {
		return nil, errors.FromDB(err, "session not found or expired")
	}
	return &session, nil
}

func (r *sessionRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&UserSession{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "session not found")
	}
	return nil
}

func (r *sessionRepository) RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return errors.FromDB(
		r.db.WithContext(ctx).
			Model(&UserSession{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", now).Error,
		"",
	)
}

func (r *sessionRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	return errors.FromDB(
		r.db.WithContext(ctx).
			Where("expires_at < ?", before).
			Delete(&UserSession{}).Error,
		"",
	)
}

// loginLogRepository implements LoginLogRepositoryPort.
type loginLogRepository struct {
	db *gorm.DB
}

func NewLoginLogRepository(db *gorm.DB) LoginLogRepositoryPort {
	return &loginLogRepository{db: db}
}

func (r *loginLogRepository) Append(ctx context.Context, log *UserLoginLog) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(log).Error, "")
}

// loginStatsRepository implements LoginStatsRepositoryPort.
type loginStatsRepository struct {
	db *gorm.DB
}

func NewLoginStatsRepository(db *gorm.DB) LoginStatsRepositoryPort {
	return &loginStatsRepository{db: db}
}

func (r *loginStatsRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*UserLoginStats, error) {
	var stats UserLoginStats
	err := r.db.WithContext(ctx).First(&stats, "user_id = ?", userID).Error
	if err != nil {
		return nil, errors.FromDB(err, "login stats not found")
	}
	return &stats, nil
}

func (r *loginStatsRepository) Upsert(ctx context.Context, stats *UserLoginStats) error {
	// OnConflict pada user_id (uniqueIndex): kalau sudah ada, update semua kolom kecuali PK.
	return errors.FromDB(
		r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"total_attempts",
					"total_failed_attempts",
					"last_attempt_at",
					"last_success_at",
					"last_failed_ip",
					"is_locked",
					"locked_at",
					"locked_reason",
					"updated_at",
				}),
			}).
			Create(stats).Error,
		"",
	)
}
