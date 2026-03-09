package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"parkieee/internal/modules/auth"
	authmocks "parkieee/internal/modules/auth/mocks"
	"parkieee/pkg/config"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
)

type noopLogger struct{}

func (noopLogger) Debug(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Info(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Warn(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Error(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Fatal(_ context.Context, _ string, _ ...any) {}
func (n noopLogger) With(_ ...any) logger.Logger               { return n }
func (n noopLogger) WithGroup(_ string) logger.Logger          { return n }

var _ logger.Logger = noopLogger{}

func testCfg() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			SecretKey:      "test-secret-32-chars-long-enough!",
			GateSecretKey:  "test-gate-secret",
			AccessTokenTTL: 15 * time.Minute,
		},
	}
}

type allMocks struct {
	user       *authmocks.MockUserRepositoryPort
	role       *authmocks.MockRoleRepositoryPort
	perm       *authmocks.MockPermissionRepositoryPort
	rolePerm   *authmocks.MockRolePermissionRepositoryPort
	session    *authmocks.MockSessionRepositoryPort
	loginLog   *authmocks.MockLoginLogRepositoryPort
	loginStats *authmocks.MockLoginStatsRepositoryPort
}

func newSvc(t *testing.T) (auth.ServicePort, allMocks) {
	m := allMocks{
		user:       authmocks.NewMockUserRepositoryPort(t),
		role:       authmocks.NewMockRoleRepositoryPort(t),
		perm:       authmocks.NewMockPermissionRepositoryPort(t),
		rolePerm:   authmocks.NewMockRolePermissionRepositoryPort(t),
		session:    authmocks.NewMockSessionRepositoryPort(t),
		loginLog:   authmocks.NewMockLoginLogRepositoryPort(t),
		loginStats: authmocks.NewMockLoginStatsRepositoryPort(t),
	}
	svc := auth.NewService(
		m.user, m.role, m.perm, m.rolePerm,
		m.session, m.loginLog, m.loginStats,
		testCfg(), noopLogger{},
	)
	return svc, m
}

// sha256hex replicates the hashToken function used inside auth/service.go.
// Kept here so tests are self-documenting about the expected hash format.
func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestCreateUser_PasswordTooShort(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.CreateUser(context.Background(), &auth.CreateUserRequest{
		Name: "Test", Email: "test@example.com", Password: "ab1", RoleID: uuid.New(),
	})
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrValidation))
}

func TestCreateUser_PasswordNoDigit(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.CreateUser(context.Background(), &auth.CreateUserRequest{
		Name: "Test", Email: "test@example.com", Password: "password", RoleID: uuid.New(),
	})
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrValidation))
}

func TestCreateUser_PasswordNoLetter(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.CreateUser(context.Background(), &auth.CreateUserRequest{
		Name: "Test", Email: "test@example.com", Password: "12345678", RoleID: uuid.New(),
	})
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrValidation))
}

func TestCreateUser_ValidPassword_ProceedsToRoleLookup(t *testing.T) {
	svc, m := newSvc(t)
	roleID := uuid.New()

	// Password valid — error seharusnya dari role not found, bukan validation
	m.role.EXPECT().
		FindByID(context.Background(), roleID).
		Return(nil, errors.New(errors.ErrNotFound, "role not found"))

	_, err := svc.CreateUser(context.Background(), &auth.CreateUserRequest{
		Name: "Test", Email: "test@example.com", Password: "pass1234", RoleID: roleID,
	})
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrNotFound), "expected role-not-found, not a validation error")
}

func TestValidateToken_InvalidJWT(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.ValidateToken(context.Background(), "not.a.valid.jwt")
	require.Error(t, err)
}

func TestValidateToken_TamperedJWT(t *testing.T) {
	svc, _ := newSvc(t)
	tampered := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.wrongsignature"
	_, err := svc.ValidateToken(context.Background(), tampered)
	require.Error(t, err)
}

func TestLogout_CallsRevokeWithCorrectHash(t *testing.T) {
	svc, m := newSvc(t)

	token := "some.valid.looking.token"
	expectedHash := sha256hex(token)

	// Logout hashes token lalu FindByTokenHash untuk mendapatkan session ID
	m.session.EXPECT().
		FindByTokenHash(context.Background(), expectedHash).
		Return(nil, errors.New(errors.ErrNotFound, "session not found"))

	// ErrNotFound pada FindByTokenHash → Logout kembalikan error (session tidak ditemukan)
	err := svc.Logout(context.Background(), token)
	require.Error(t, err)
}

func TestLogout_RevokesCorrectSession(t *testing.T) {
	svc, m := newSvc(t)

	token := "a.valid.session.token"
	sessionID := uuid.New()
	userID := uuid.New()
	expectedHash := sha256hex(token)

	m.session.EXPECT().
		FindByTokenHash(context.Background(), expectedHash).
		Return(&auth.UserSession{
			ID:     sessionID,
			UserID: userID,
		}, nil)

	m.session.EXPECT().
		Revoke(context.Background(), sessionID).
		Return(nil)

	m.loginLog.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(_ *auth.UserLoginLog) bool { return true })).
		Return(nil).
		Maybe()

	err := svc.Logout(context.Background(), token)
	require.NoError(t, err)
}
