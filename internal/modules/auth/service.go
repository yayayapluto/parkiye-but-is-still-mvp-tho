	package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
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

const sessionCacheTTL = 60 * time.Second

type cachedSession struct {
	claims    *middleware.TokenClaims
	cachedAt  time.Time
	expiresAt time.Time
}

type service struct {
	userRepo         UserRepositoryPort
	roleRepo         RoleRepositoryPort
	permRepo         PermissionRepositoryPort
	rolePermRepo     RolePermissionRepositoryPort
	sessionRepo      SessionRepositoryPort
	refreshTokenRepo RefreshTokenRepositoryPort
	loginLogRepo     LoginLogRepositoryPort
	loginStatsRepo   LoginStatsRepositoryPort
	cfg              *config.Config
	log              logger.Logger
	sessionCache     sync.Map // key: tokenHash(string) → cachedSession
	userRevokedAt    sync.Map // key: userID(string) → time.Time — kapan semua session user di-revoke
}

func NewService(
	userRepo UserRepositoryPort,
	roleRepo RoleRepositoryPort,
	permRepo PermissionRepositoryPort,
	rolePermRepo RolePermissionRepositoryPort,
	sessionRepo SessionRepositoryPort,
	refreshTokenRepo RefreshTokenRepositoryPort,
	loginLogRepo LoginLogRepositoryPort,
	loginStatsRepo LoginStatsRepositoryPort,
	cfg *config.Config,
	log logger.Logger,
) ServicePort {
	return &service{
		userRepo:         userRepo,
		roleRepo:         roleRepo,
		permRepo:         permRepo,
		rolePermRepo:     rolePermRepo,
		sessionRepo:      sessionRepo,
		refreshTokenRepo: refreshTokenRepo,
		loginLogRepo:     loginLogRepo,
		loginStatsRepo:   loginStatsRepo,
		cfg:              cfg,
		log:              log,
	}
}

func (s *service) Login(ctx context.Context, identifier, password, ip, userAgent string, remember bool) (*LoginResponse, error) {
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

	// Deteksi apakah identifier adalah email (mengandung @) atau username
	var user *User
	var err error
	if strings.Contains(identifier, "@") {
		user, err = s.userRepo.FindByEmail(ctx, identifier)
	} else {
		user, err = s.userRepo.FindByUsername(ctx, identifier)
	}
	if err != nil {
		// Jangan expose "user not found" ke client — selalu kasih pesan generic
		return fail(types.FailureUserNotFound, errors.New(errors.ErrInvalidCredentials, "Email/username atau kata sandi salah"))
	}

	logEntry.UserID = &user.ID

	if !user.IsActive || user.DeletedAt != nil {
		return fail(types.FailureAccountInactive, errors.New(errors.ErrUnauthorized, "Akun tidak aktif"))
	}

	// Cek lockout sebelum verifikasi password
	stats, err := s.loginStatsRepo.FindByUserID(ctx, user.ID)
	if err == nil && stats.IsLocked {
		return fail(types.FailureAccountLocked, errors.New(errors.ErrAccountLocked, "Akun terkunci, hubungi administrator"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.recordFailedAttempt(ctx, user.ID, ip, stats)
		return fail(types.FailureWrongPassword, errors.New(errors.ErrInvalidCredentials, "invalid email or password"))
	}

	// Hapus session expired sebelum buat yang baru — housekeeping ringan
	_ = s.sessionRepo.DeleteExpired(ctx, time.Now())

	ttl := s.cfg.JWT.AccessTokenTTL
	if remember {
		ttl = 7 * 24 * time.Hour // 7 hari
	}

	token, expiresAt, err := s.generateJWT(user, ttl)
	if err != nil {
		s.log.Error(ctx, "failed to generate JWT", "error", err, "user_id", user.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat token")
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

	rawRefresh, refreshExpiresAt, err := s.generateRefreshToken()
	if err != nil {
		s.log.Error(ctx, "failed to generate refresh token", "error", err, "user_id", user.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat refresh token")
	}
	rt := &RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		SessionID: session.ID,
		TokenHash: hashToken(rawRefresh),
		IPAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: refreshExpiresAt,
	}
	if err := s.refreshTokenRepo.Create(ctx, rt); err != nil {
		return nil, err
	}

	logEntry.Success = true
	_ = s.loginLogRepo.Append(ctx, logEntry)

	s.resetLoginStats(ctx, user.ID)
	s.log.Info(ctx, "user logged in", "user_id", user.ID, "email", user.Email, "ip", ip)

	return &LoginResponse{Token: token, ExpiresAt: expiresAt, RefreshToken: rawRefresh, RefreshExpiresAt: refreshExpiresAt, User: toUserResponse(user)}, nil
}

func (s *service) Logout(ctx context.Context, token string) error {
	session, err := s.sessionRepo.FindByTokenHash(ctx, hashToken(token))
	if err != nil {
		s.log.Warn(ctx, "logout failed: session not found")
		return err
	}

	if err := s.sessionRepo.Revoke(ctx, session.ID); err != nil {
		s.log.Error(ctx, "failed to revoke session", "session_id", session.ID, "user_id", session.UserID, "error", err)
		return err
	}

	// Revoke refresh token yang terkait sesi ini sekaligus
	_ = s.refreshTokenRepo.RevokeBySessionID(ctx, session.ID)

	// Invalidate cache entry supaya access token langsung ditolak
	s.sessionCache.Delete(hashToken(token))

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

func (s *service) Refresh(ctx context.Context, rawRefreshToken, ip, userAgent string) (*LoginResponse, error) {
	rt, err := s.refreshTokenRepo.FindByTokenHash(ctx, hashToken(rawRefreshToken))
	if err != nil {
		s.log.Warn(ctx, "refresh failed: token not found or expired")
		return nil, errors.New(errors.ErrUnauthorized, "Refresh token tidak valid atau sudah kedaluwarsa")
	}

	user, err := s.userRepo.FindByID(ctx, rt.UserID)
	if err != nil || !user.IsActive || user.DeletedAt != nil {
		s.log.Warn(ctx, "refresh failed: user not found or inactive", "user_id", rt.UserID)
		return nil, errors.New(errors.ErrUnauthorized, "Akun tidak ditemukan atau tidak aktif")
	}

	newToken, newExpiresAt, err := s.generateJWT(user, s.cfg.JWT.AccessTokenTTL)
	if err != nil {
		s.log.Error(ctx, "refresh: failed to generate access token", "error", err, "user_id", user.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat token")
	}

	newRawRefresh, newRefreshExpiresAt, err := s.generateRefreshToken()
	if err != nil {
		s.log.Error(ctx, "refresh: failed to generate refresh token", "error", err, "user_id", user.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat refresh token")
	}

	newSession := &UserSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(newToken),
		IPAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: newExpiresAt,
	}
	if err := s.sessionRepo.Create(ctx, newSession); err != nil {
		return nil, err
	}

	newRT := &RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		SessionID: newSession.ID,
		TokenHash: hashToken(newRawRefresh),
		IPAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: newRefreshExpiresAt,
	}
	if err := s.refreshTokenRepo.Create(ctx, newRT); err != nil {
		return nil, err
	}

	// Revoke token lama dan catat ID penggantinya (refresh token rotation)
	if err := s.refreshTokenRepo.Revoke(ctx, rt.ID, &newRT.ID); err != nil {
		s.log.Error(ctx, "refresh: failed to revoke old token", "error", err, "rt_id", rt.ID)
		return nil, errors.New(errors.ErrInternal, "Gagal merotasi refresh token")
	}
	// Revoke sesi lama juga
	_ = s.sessionRepo.Revoke(ctx, rt.SessionID)
	s.sessionCache.Delete(hashToken(rawRefreshToken))

	s.log.Info(ctx, "token refreshed", "user_id", user.ID)
	return &LoginResponse{Token: newToken, ExpiresAt: newExpiresAt, RefreshToken: newRawRefresh, RefreshExpiresAt: newRefreshExpiresAt, User: toUserResponse(user)}, nil
}

func (s *service) ValidateToken(ctx context.Context, token string) (*middleware.TokenClaims, error) {
	claims, err := s.parseJWT(token)
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
	}

	h := hashToken(token)

	// Cache hit — skip DB lookup
	if v, ok := s.sessionCache.Load(h); ok {
		entry := v.(cachedSession)
		if time.Now().Before(entry.expiresAt) {
			// Cek apakah semua session user ini sudah di-revoke setelah cache entry dibuat
			if rv, rOK := s.userRevokedAt.Load(entry.claims.UserID.String()); rOK {
				revokedAt := rv.(time.Time)
				if revokedAt.After(entry.cachedAt) {
					s.sessionCache.Delete(h)
					s.log.Warn(ctx, "validate token: session revoked (cache hit)", "user_id", entry.claims.UserID)
					return nil, errors.New(errors.ErrUnauthorized, "Sesi telah berakhir atau dicabut")
				}
			}
			s.log.Debug(ctx, "validate token: cache hit", "user_id", entry.claims.UserID)
			return entry.claims, nil
		}
		// Expired — hapus dari cache dan lanjut ke DB
		s.sessionCache.Delete(h)
	}

	// Cache miss — query DB
	_, err = s.sessionRepo.FindByTokenHash(ctx, h)
	if err != nil {
		s.log.Warn(ctx, "validate token: session not found in DB (cache miss)")
		return nil, errors.New(errors.ErrUnauthorized, "Sesi telah berakhir atau dicabut")
	}

	// Simpan ke cache
	now := time.Now()
	s.sessionCache.Store(h, cachedSession{
		claims:    claims,
		cachedAt:  now,
		expiresAt: now.Add(sessionCacheTTL),
	})

	s.log.Debug(ctx, "validate token: cache miss, loaded from DB", "user_id", claims.UserID)
	return claims, nil
}

func (s *service) ListUsers(ctx context.Context, roleID *uuid.UUID, activeOnly *bool, page, pageSize int) ([]User, int64, error) {
	return s.userRepo.List(ctx, roleID, activeOnly, page, pageSize)
}

func (s *service) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

func (s *service) GetProfile(ctx context.Context, userID uuid.UUID) (*User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		s.log.Warn(ctx, "get profile: user not found", "user_id", userID)
		return nil, err
	}
	s.log.Debug(ctx, "get profile", "user_id", userID)
	return user, nil
}

func (s *service) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if err := validatePasswordComplexity(req.Password); err != nil {
		return nil, err
	}

	if _, err := s.roleRepo.FindByID(ctx, req.RoleID); err != nil {
		s.log.Warn(ctx, "create user failed: role not found", "role_id", req.RoleID)
		return nil, errors.New(errors.ErrNotFound, "Role tidak ditemukan")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error(ctx, "failed to hash password for new user", "email", req.Email, "error", err)
		return nil, errors.New(errors.ErrInternal, "Gagal membuat hash kata sandi")
	}

	user := &User{
		ID:           uuid.New(),
		Name:         req.Name,
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		RoleID:       req.RoleID,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.log.Error(ctx, "failed to create user", "email", req.Email, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "user created", "user_id", user.ID, "email", user.Email)
	return s.userRepo.FindByID(ctx, user.ID)
}

func (s *service) UpdateUser(ctx context.Context, userID uuid.UUID, req *UpdateUserRequest) (*User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		s.log.Warn(ctx, "update user failed: user not found", "user_id", userID)
		return nil, err
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.RoleID != nil {
		if _, err := s.roleRepo.FindByID(ctx, *req.RoleID); err != nil {
			return nil, errors.New(errors.ErrNotFound, "Role tidak ditemukan")
		}
		user.RoleID = *req.RoleID
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.log.Error(ctx, "failed to update user", "user_id", userID, "error", err)
		return nil, err
	}
	s.log.Info(ctx, "user updated", "user_id", userID)
	return s.userRepo.FindByID(ctx, userID)
}

func (s *service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	if err := validatePasswordComplexity(newPassword); err != nil {
		return err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		s.log.Warn(ctx, "change password failed: user not found", "user_id", userID)
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		s.log.Warn(ctx, "change password failed: wrong current password", "user_id", userID)
		return errors.New(errors.ErrInvalidCredentials, "Kata sandi saat ini tidak benar")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error(ctx, "failed to hash new password", "user_id", userID, "error", err)
		return errors.New(errors.ErrInternal, "failed to hash password")
	}

	user.PasswordHash = string(hash)
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.log.Error(ctx, "failed to save new password", "user_id", userID, "error", err)
		return err
	}

	// Revoke semua session supaya user login ulang di semua device
	if err := s.sessionRepo.RevokeAllByUserID(ctx, userID); err != nil {
		s.log.Error(ctx, "failed to revoke sessions after password change", "user_id", userID, "error", err)
	}
	// Tandai semua cache entry milik user ini sebagai tidak valid
	s.userRevokedAt.Store(userID.String(), time.Now())

	s.log.Info(ctx, "password changed", "user_id", userID)
	return nil
}

func (s *service) DeactivateUser(ctx context.Context, userID uuid.UUID, actorID uuid.UUID) error {
	if userID == actorID {
		s.log.Warn(ctx, "deactivate user rejected: cannot self-deactivate", "user_id", userID, "actor_id", actorID)
		return errors.New(errors.ErrForbidden, "Tidak dapat menonaktifkan akun Anda sendiri")
	}

	if _, err := s.userRepo.FindByID(ctx, userID); err != nil {
		s.log.Warn(ctx, "deactivate user failed: user not found", "user_id", userID)
		return err
	}

	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		s.log.Error(ctx, "failed to deactivate user", "user_id", userID, "actor_id", actorID, "error", err)
		return err
	}

	_ = s.sessionRepo.RevokeAllByUserID(ctx, userID)
	// Tandai semua cache entry milik user ini sebagai tidak valid
	s.userRevokedAt.Store(userID.String(), time.Now())

	s.log.Info(ctx, "user deactivated", "user_id", userID, "actor_id", actorID)
	return nil
}

func (s *service) GetAllRoles(ctx context.Context) ([]Role, error) {
	roles, err := s.roleRepo.FindAll(ctx)
	if err != nil {
		s.log.Error(ctx, "get all roles: db error", "error", err)
		return nil, err
	}
	s.log.Debug(ctx, "get all roles", "count", len(roles))
	return roles, nil
}

func (s *service) GetAllPermissions(ctx context.Context) ([]Permission, error) {
	perms, err := s.permRepo.FindAll(ctx)
	if err != nil {
		s.log.Error(ctx, "get all permissions: db error", "error", err)
		return nil, err
	}
	s.log.Debug(ctx, "get all permissions", "count", len(perms))
	return perms, nil
}

func (s *service) AssignPermission(ctx context.Context, roleID, permissionID uuid.UUID, actorID uuid.UUID) error {
	if _, err := s.roleRepo.FindByID(ctx, roleID); err != nil {
		s.log.Warn(ctx, "assign permission failed: role not found", "role_id", roleID)
		return errors.New(errors.ErrNotFound, "role not found")
	}
	if _, err := s.permRepo.FindByID(ctx, permissionID); err != nil {
		s.log.Warn(ctx, "assign permission failed: permission not found", "permission_id", permissionID)
		return errors.New(errors.ErrNotFound, "Izin tidak ditemukan")
	}

	if err := s.rolePermRepo.Assign(ctx, roleID, permissionID, actorID); err != nil {
		s.log.Error(ctx, "failed to assign permission", "role_id", roleID, "permission_id", permissionID, "error", err)
		return err
	}

	s.log.Info(ctx, "permission assigned", "role_id", roleID, "permission_id", permissionID, "actor_id", actorID)
	return nil
}

func (s *service) RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID, actorID uuid.UUID) error {
	if err := s.rolePermRepo.Revoke(ctx, roleID, permissionID); err != nil {
		s.log.Error(ctx, "failed to revoke permission", "role_id", roleID, "permission_id", permissionID, "error", err)
		return err
	}

	s.log.Info(ctx, "permission revoked", "role_id", roleID, "permission_id", permissionID, "actor_id", actorID)
	return nil
}

func (s *service) CheckPermission(ctx context.Context, userID uuid.UUID, node string) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		s.log.Warn(ctx, "check permission: user not found", "user_id", userID, "node", node)
		return false, err
	}
	has, err := s.rolePermRepo.HasPermission(ctx, user.RoleID, node)
	if err != nil {
		s.log.Error(ctx, "check permission: db error", "user_id", userID, "node", node, "error", err)
		return false, err
	}
	s.log.Debug(ctx, "check permission", "user_id", userID, "node", node, "granted", has)
	return has, nil
}

func (s *service) generateJWT(user *User, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)

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
			return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid")
		}
		return []byte(s.cfg.JWT.SecretKey), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid")
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid")
	}

	subStr, _ := claims["sub"].(string)
	if subStr == "" {
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid")
	}
	userID, err := uuid.Parse(subStr)
	if err != nil {
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid")
	}

	perms, _ := claims["permissions"].([]interface{})
	permStrings := make([]string, 0, len(perms))
	for _, p := range perms {
		if s, ok := p.(string); ok {
			permStrings = append(permStrings, s)
		}
	}

	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)
	if email == "" || role == "" {
		return nil, errors.New(errors.ErrUnauthorized, "Token tidak valid")
	}

	return &middleware.TokenClaims{
		UserID:      userID,
		Email:       email,
		Role:        role,
		Permissions: permStrings,
	}, nil
}

// validatePasswordComplexity memastikan password minimal 8 karakter, ada huruf dan angka.
func validatePasswordComplexity(password string) error {
	if len(password) < 8 {
		return errors.New(errors.ErrValidation, "Kata sandi minimal 8 karakter")
	}
	hasLetter, hasDigit := false, false
	for _, c := range password {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return nil
		}
	}
	if !hasLetter {
		return errors.New(errors.ErrValidation, "Kata sandi harus mengandung minimal satu huruf")
	}
	return errors.New(errors.ErrValidation, "Kata sandi harus mengandung minimal satu angka")
}

// hashToken SHA-256 token sebelum disimpan ke DB — token asli tidak pernah disimpan.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// generateRefreshToken membuat 32-byte random token yang dikodekan hex.
func (s *service) generateRefreshToken() (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	return hex.EncodeToString(b), time.Now().Add(s.cfg.JWT.RefreshTokenTTL), nil
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
