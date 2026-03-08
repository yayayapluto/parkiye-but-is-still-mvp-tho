package gate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

type pairingRepository struct {
	db         *gorm.DB
	mu         sync.RWMutex
	sseClients map[string]chan string // code → channel SSE aktif
}

func NewPairingRepository(db *gorm.DB) PairingRepositoryPort {
	return &pairingRepository{
		db:         db,
		sseClients: make(map[string]chan string),
	}
}

func (r *pairingRepository) Create(ctx context.Context, p *GatePairingCode) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(p).Error, "")
}

func (r *pairingRepository) FindByCode(ctx context.Context, code string) (*GatePairingCode, error) {
	var p GatePairingCode
	err := r.db.WithContext(ctx).First(&p, "code = ?", code).Error
	return &p, errors.FromDB(err, "pairing code not found")
}

func (r *pairingRepository) InvalidatePendingByIP(ctx context.Context, ip string) error {
	return errors.FromDB(
		r.db.WithContext(ctx).
			Model(&GatePairingCode{}).
			Where("ip_address = ? AND status = 'pending' AND expires_at > NOW()", ip).
			Update("expires_at", time.Now()).
			Error,
		"",
	)
}

func (r *pairingRepository) Confirm(ctx context.Context, id uuid.UUID, gateID uuid.UUID, confirmedBy uuid.UUID, gateJWT string) error {
	// Simpan hash SHA-256 dari JWT, bukan plaintext.
	// JWT asli hanya dikirim ke screen via SSE channel — tidak pernah disimpan ke DB.
	h := sha256.Sum256([]byte(gateJWT))
	jwtHash := hex.EncodeToString(h[:])

	now := time.Now()
	return errors.FromDB(
		r.db.WithContext(ctx).
			Model(&GatePairingCode{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":       "confirmed",
				"gate_id":      gateID,
				"confirmed_by": confirmedBy,
				"confirmed_at": now,
				"gate_jwt":     jwtHash,
			}).Error,
		"",
	)
}

func (r *pairingRepository) SetSSEClient(code string, ch chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Kick koneksi lama kalau ada
	if old, ok := r.sseClients[code]; ok {
		close(old)
	}
	r.sseClients[code] = ch
}

func (r *pairingRepository) GetSSEClient(code string) (chan string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.sseClients[code]
	return ch, ok
}

func (r *pairingRepository) RemoveSSEClient(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sseClients, code)
}

// generatePairingCode menghasilkan 6 alphanumeric random (A-Z0-9).
func generatePairingCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	rb := make([]byte, 6)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(rb[i])%len(charset)]
	}
	return string(b), nil
}

// buildQRContent menghasilkan JSON string yang di-encode ke QR code.
// Format: { "url": "...", "expires_at": "..." }
func buildQRContent(baseURL, code string, expiresAt time.Time) (string, error) {
	data := map[string]string{
		"url":        fmt.Sprintf("%s/pair/%s", baseURL, code),
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// encodeQRBase64 menghasilkan base64 dari QR content — placeholder untuk generate QR image.
// Di production bisa pakai library QR seperti github.com/skip2/go-qrcode.
// Untuk sekarang kita return base64 dari JSON content-nya langsung
// supaya frontend bisa generate QR sendiri dari string ini.
func encodeQRBase64(content string) string {
	return base64.StdEncoding.EncodeToString([]byte(content))
}
