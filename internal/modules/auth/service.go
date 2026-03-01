package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"parkieee/pkg/config"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/middleware"
	"parkieee/pkg/types"
)

type service struct {
	userRepo       UserRepositoryPort
	roleRepo       RoleRepositoryPort
	permRepo       PermissionRepositoryPort
	rolePermRepo   RolePermissionRepositoryPort
	sessionRepo    SessionRepositoryPort
	loginLogRepo   LoginLogRepositoryPort
	loginStatsRepo LoginStatsRepositoryPort
	cfg            *config.Config
	log            logger.Logger
}

func NewService(
	userRepo UserRepositoryPort,
	roleRepo RoleRepositoryPort,
	permRepo PermissionRepositoryPort,
	rolePermRepo RolePermissionRepositoryPort,
	sessionRepo SessionRepositoryPort,
	loginLogRepo LoginLogRepositoryPort,
	loginStatsRepo LoginStatsRepositoryPort,
	cfg *config.Config,
	log logger.Logger,
) ServicePort {
	return &service{
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		permRepo:       permRepo,
		rolePermRepo:   rolePermRepo,
		sessionRepo:    sessionRepo,
		loginLogRepo:   loginLogRepo,
		loginStatsRepo: loginStatsRepo,
		cfg:            cfg,
		log:            log,
	}
}

func (s *service) Login(ctx context.Context, email, password, ip, userAgent string) (*LoginResponse, error) {
	logEntry := &UserLoginLog{
		IPAddress:   ip,
		UserAgent:   userAgent,
		AttemptType: types.LoginAttemptLogin,
		AttemptedAt: time.Now(),
	}

	fail := func(reason types.FailureReason, err error) (*LoginResponse, error) {
		logEntry.Success = false
		logEntry.FailureReason = &reason
		_ = s.loginLogRepo.Append(ctx, logEntry)
		return nil, err
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// Jangan expose "user not found" ke client — selalu kasih pesan generic
		return fail(types.FailureUserNotFound, errors.New(errors.ErrInvalidCredentials, "invalid email or password"))
	}

	logEntry.UserID = &user.ID

	if !user.IsActive || user.DeletedAt != nil {
		return fail(types.FailureAccountInactive, errors.New(errors.ErrUnauthorized, "account is inactive"))
	}

	// Cek lockout sebelum verifikasi password
	stats, err := s.loginStatsRepo.FindByUserID(ctx, user.ID)
	if err == nil && stats.IsLocked {
		return fail(types.FailureAccountLocked, errors.New(errors.ErrAccountLocked, "account is locked, contact administrator"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.recordFailedAttempt(ctx, user.ID, ip, stats)
		return fail(types.FailureWrongPassword, errors.New(errors.ErrInvalidCredentials, "invalid email or password"))
	}

	// Hapus session expired sebelum buat yang baru — housekeeping ringan
	_ = s.sessionRepo.DeleteExpired(ctx, time.Now())

	token, expiresAt, err := s.generateJWT(user)
	if err != nil {
		s.log.Error(ctx, "failed to generate JWT", "error", err, "user_id", user.ID)
		return nil, errors.New(errors.ErrInternal, "failed to generate token")
	}

	session := &UserSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(token),
		IPAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	logEntry.Success = true
	_ = s.loginLogRepo.Append(ctx, logEntry)

	s.resetLoginStats(ctx, user.ID)
	s.log.Info(ctx, "user logged in", "user_id", user.ID, "email", user.Email, "ip", ip)

	return &LoginResponse{Token: token, ExpiresAt: expiresAt, User: toUserResponse(user)}, nil
}

func (s *service) Logout(ctx context.Context, token string) error {
	session, err := s.sessionRepo.FindByTokenHash(ctx, hashToken(token))
	if err != nil {
		return err
	}

	if err := s.sessionRepo.Revoke(ctx, session.ID); err != nil {
		return err
	}

	log := &UserLoginLog{
		UserID:      &session.UserID,
		AttemptType: types.LoginAttemptLogout,
		Success:     true,
		AttemptedAt: time.Now(),
	}
	_ = s.loginLogRepo.Append(ctx, log)

	s.log.Info(ctx, "user logged out", "user_id", session.UserID)
	return nil
}

func (s *service) ValidateToken(ctx context.Context, token string) (*middleware.TokenClaims, error) {
	claims, err := s.parseJWT(token)
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "invalid or expired token")
	}

	// Cek session masih aktif di DB — token valid tapi session bisa sudah di-revoke
	_, err = s.sessionRepo.FindByTokenHash(ctx, hashToken(token))
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "session expired or revoked")
	}

	return claims, nil
}

func (s *service) GetProfile(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

func (s *service) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if _, err := s.roleRepo.FindByID(ctx, req.RoleID); err != nil {
		return nil, errors.New(errors.ErrNotFound, "role not found")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New(errors.ErrInternal, "failed to hash password")
	}

	user := &User{
		ID:           uuid.New(),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		RoleID:       req.RoleID,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	s.log.Info(ctx, "user created", "user_id", user.ID, "email", user.Email)
	return s.userRepo.FindByID(ctx, user.ID)
}

func (s *service) UpdateUser(ctx context.Context, userID uuid.UUID, req *UpdateUserRequest) (*User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.RoleID != nil {
		if _, err := s.roleRepo.FindByID(ctx, *req.RoleID); err != nil {
			return nil, errors.New(errors.ErrNotFound, "role not found")
		}
		user.RoleID = *req.RoleID
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return s.userRepo.FindByID(ctx, userID)
}

func (s *service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New(errors.ErrInvalidCredentials, "current password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New(errors.ErrInternal, "failed to hash password")
	}

	user.PasswordHash = string(hash)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Revoke semua session supaya user login ulang di semua device
	if err := s.sessionRepo.RevokeAllByUserID(ctx, userID); err != nil {
		s.log.Error(ctx, "failed to revoke sessions after password change", "user_id", userID, "error", err)
	}

	s.log.Info(ctx, "password changed", "user_id", userID)
	return nil
}

func (s *service) DeactivateUser(ctx context.Context, userID uuid.UUID, actorID uuid.UUID) error {
	if userID == actorID {
		return errors.New(errors.ErrForbidden, "cannot deactivate your own account")
	}

	if _, err := s.userRepo.FindByID(ctx, userID); err != nil {
		return err
	}

	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		return err
	}

	_ = s.sessionRepo.RevokeAllByUserID(ctx, userID)

	s.log.Info(ctx, "user deactivated", "user_id", userID, "actor_id", actorID)
	return nil
}

func (s *service) GetAllRoles(ctx context.Context) ([]Role, error) {
	return s.roleRepo.FindAll(ctx)
}

func (s *service) GetAllPermissions(ctx context.Context) ([]Permission, error) {
	return s.permRepo.FindAll(ctx)
}

func (s *service) AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID, actorID uuid.UUID) error {
	if _, err := s.roleRepo.FindByID(ctx, roleID); err != nil {
		return errors.New(errors.ErrNotFound, "role not found")
	}
	if _, err := s.permRepo.FindByID(ctx, permissionID); err != nil {
		return errors.New(errors.ErrNotFound, "permission not found")
	}

	if err := s.rolePermRepo.Assign(ctx, roleID, permissionID, actorID); err != nil {
		return err
	}

	s.log.Info(ctx, "permission assigned", "role_id", roleID, "permission_id", permissionID, "actor_id", actorID)
	return nil
}

func (s *service) RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID, actorID uuid.UUID) error {
	if err := s.rolePermRepo.Revoke(ctx, roleID, permissionID); err != nil {
		return err
	}

	s.log.Info(ctx, "permission revoked", "role_id", roleID, "permission_id", permissionID, "actor_id", actorID)
	return nil
}

func (s *service) CheckPermission(ctx context.Context, userID uuid.UUID, node string) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return s.rolePermRepo.HasPermission(ctx, user.RoleID, node)
}

func (s *service) generateJWT(user *User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.cfg.JWT.AccessTokenTTL)

	perms := make([]string, 0)
	for _, rp := range user.Role.Permissions {
		if rp.Permission != nil {
			perms = append(perms, rp.Permission.Node)
		}
	}

	claims := jwt.MapClaims{
		"sub":         user.ID.String(),
		"email":       user.Email,
		"role":        user.Role.Name,
		"permissions": perms,
		"exp":         expiresAt.Unix(),
		"iat":         time.Now().Unix(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(s.cfg.JWT.SecretKey))
	return token, expiresAt, err
}

func (s *service) parseJWT(tokenStr string) (*middleware.TokenClaims, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(errors.ErrUnauthorized, "unexpected signing method")
		}
		return []byte(s.cfg.JWT.SecretKey), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New(errors.ErrUnauthorized, "invalid token")
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New(errors.ErrUnauthorized, "invalid token claims")
	}

	userID, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "invalid token subject")
	}

	perms, _ := claims["permissions"].([]interface{})
	permStrings := make([]string, 0, len(perms))
	for _, p := range perms {
		if s, ok := p.(string); ok {
			permStrings = append(permStrings, s)
		}
	}

	return &middleware.TokenClaims{
		UserID:      userID,
		Email:       claims["email"].(string),
		Role:        claims["role"].(string),
		Permissions: permStrings,
	}, nil
}

// hashToken SHA-256 token sebelum disimpan ke DB — token asli tidak pernah disimpan.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// recordFailedAttempt update stats dan lock akun kalau sudah 5x gagal berturut-turut.
func (s *service) recordFailedAttempt(ctx context.Context, userID uuid.UUID, ip string, stats *UserLoginStats) {
	now := time.Now()

	if stats == nil {
		stats = &UserLoginStats{UserID: userID}
	}

	stats.TotalAttempts++
	stats.TotalFailedAttempts++
	stats.LastAttemptAt = &now
	stats.LastFailedIP = &ip

	// Lock setelah 5 kali gagal
	if stats.TotalFailedAttempts >= 5 {
		stats.IsLocked = true
		stats.LockedAt = &now
		reason := types.LockedReasonTooManyFailedAttempts
		stats.LockedReason = &reason
		s.log.Info(ctx, "account locked due to too many failed attempts", "user_id", userID)
	}

	_ = s.loginStatsRepo.Upsert(ctx, stats)
}

// resetLoginStats reset counter setelah login sukses.
func (s *service) resetLoginStats(ctx context.Context, userID uuid.UUID) {
	now := time.Now()
	stats := &UserLoginStats{
		UserID:              userID,
		TotalFailedAttempts: 0,
		IsLocked:            false,
		LastSuccessAt:       &now,
		LastAttemptAt:       &now,
	}
	_ = s.loginStatsRepo.Upsert(ctx, stats)
}
