package transaction

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
	"parkieee/pkg/types"
)

type transactionRepo struct{ db *gorm.DB }
type transactionLogRepo struct{ db *gorm.DB }

func NewTransactionRepository(db *gorm.DB) TransactionRepositoryPort {
	return &transactionRepo{db}
}

func NewTransactionLogRepository(db *gorm.DB) TransactionLogRepositoryPort {
	return &transactionLogRepo{db}
}

func (r *transactionRepo) FindByID(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	var t Transaction
	err := r.db.WithContext(ctx).First(&t, "id = ?", id).Error
	return &t, errors.FromDB(err, "transaction not found")
}

func (r *transactionRepo) FindByCode(ctx context.Context, code string) (*Transaction, error) {
	var t Transaction
	err := r.db.WithContext(ctx).Where("transaction_code = ?", code).First(&t).Error
	return &t, errors.FromDB(err, "transaction not found")
}

func (r *transactionRepo) FindAll(ctx context.Context, filter ListFilter, page, pageSize int) ([]Transaction, int64, int64, error) {
	var txs []Transaction
	var total int64
	var maxAmount int64

	q := r.db.WithContext(ctx).Model(&Transaction{})

	// Filters
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if filter.ZoneID != nil {
		q = q.Where("zone_id = ?", *filter.ZoneID)
	}
	if filter.EntryGateID != nil {
		q = q.Where("entry_gate_id = ?", *filter.EntryGateID)
	}
	if filter.EntryMethod != nil {
		q = q.Where("entry_method = ?", *filter.EntryMethod)
	}
	if filter.DateFrom != nil {
		q = q.Where("entry_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		q = q.Where("entry_at <= ?", *filter.DateTo)
	}
	if filter.PlateMismatch != nil {
		q = q.Where("plate_mismatch = ?", *filter.PlateMismatch)
	}
	// Search (Partial match code or plate)
	if filter.Search != "" {
		q = q.Joins("LEFT JOIN vehicles v ON v.id = transactions.vehicle_id").
			Where("transaction_code ILIKE ? OR v.plate_number ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	// Unclosed Filter
	if filter.IsUnclosed != nil {
		if *filter.IsUnclosed {
			q = q.Where("EXISTS (SELECT 1 FROM unclosed_transaction_flags f WHERE f.transaction_id = transactions.id AND f.resolved = false)")
		} else {
			q = q.Where("NOT EXISTS (SELECT 1 FROM unclosed_transaction_flags f WHERE f.transaction_id = transactions.id AND f.resolved = false)")
		}
	}

	// Calculate Max Amount from the current filtered set (before fee range pagination)
	// We use a separate session because we don't want to include the fee filter in the max amount calculation,
	// otherwise the slider's upper limit will shrink as the user filters.
	if err := q.Session(&gorm.Session{}).Select("COALESCE(MAX(calculated_fee), 0)").Scan(&maxAmount).Error; err != nil {
		return nil, 0, 0, errors.FromDB(err, "failed to calculate max amount")
	}

	if filter.FeeMin != nil {
		q = q.Where("calculated_fee >= ?", *filter.FeeMin)
	}
	if filter.FeeMax != nil {
		q = q.Where("calculated_fee <= ?", *filter.FeeMax)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, 0, errors.FromDB(err, "")
	}

	// Sorting
	sortCol := "entry_at" // default
	sortOrder := "DESC"    // default

	// Whitelist allowed sort columns
	allowed := map[string]string{
		"entry_at":       "entry_at",
		"exit_at":        "exit_at",
		"code":             "transaction_code",
		"transaction_code": "transaction_code",
		"status":           "status",
		"fee":              "calculated_fee",
		"calculated_fee":   "calculated_fee",
		"plate_mismatch":   "plate_mismatch",
	}

	if dbCol, ok := allowed[filter.SortBy]; ok {
		sortCol = dbCol
	}

	if strings.ToLower(filter.SortOrder) == "asc" {
		sortOrder = "ASC"
	}

	offset := (page - 1) * pageSize
	err := q.Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&txs).Error
	return txs, total, maxAmount, errors.FromDB(err, "")
}

func (r *transactionRepo) FindOpenByRFIDCard(ctx context.Context, cardID uuid.UUID) (*Transaction, error) {
	var t Transaction
	err := r.db.WithContext(ctx).
		Where("rfid_card_id = ? AND status = ?", cardID, types.TransactionStatusOpen).
		First(&t).Error
	if err != nil {
		return nil, errors.FromDB(err, "open transaction not found")
	}
	return &t, nil
}

func (r *transactionRepo) FindAwaitingPaymentByRFIDCard(ctx context.Context, cardID uuid.UUID) (*Transaction, error) {
	var t Transaction
	err := r.db.WithContext(ctx).
		Where("rfid_card_id = ? AND status = ?", cardID, types.TransactionStatusAwaitingPayment).
		Order("exit_at DESC").
		First(&t).Error
	if err != nil {
		return nil, errors.FromDB(err, "awaiting payment transaction not found")
	}
	return &t, nil
}

func (r *transactionRepo) CountByDatePrefix(ctx context.Context, db *gorm.DB, prefix string) (int64, error) {
	var count int64
	err := db.WithContext(ctx).
		Model(&Transaction{}).
		Where("transaction_code LIKE ?", prefix+"%").
		Count(&count).Error
	return count, errors.FromDB(err, "")
}

// Create uses the passed db (may be a *gorm.DB transaction) so it runs atomically with siblings.
func (r *transactionRepo) Create(ctx context.Context, db *gorm.DB, t *Transaction) error {
	return errors.FromDB(db.WithContext(ctx).Create(t).Error, "")
}

// Update uses the passed db for the same reason.
func (r *transactionRepo) Update(ctx context.Context, db *gorm.DB, t *Transaction) error {
	return errors.FromDB(db.WithContext(ctx).Save(t).Error, "")
}

func (r *transactionLogRepo) FindByTransactionID(ctx context.Context, txID uuid.UUID) ([]TransactionLog, error) {
	var logs []TransactionLog
	err := r.db.WithContext(ctx).
		Where("transaction_id = ?", txID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, errors.FromDB(err, "")
}

func (r *transactionLogRepo) Append(ctx context.Context, db *gorm.DB, log *TransactionLog) error {
	log.ID = uuid.New()
	log.CreatedAt = time.Now()
	return errors.FromDB(db.WithContext(ctx).Create(log).Error, "")
}
