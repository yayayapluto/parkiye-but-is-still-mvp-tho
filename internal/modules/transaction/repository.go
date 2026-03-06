package transaction

import (
	"context"
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

func (r *transactionRepo) FindAll(ctx context.Context, filter ListFilter, page, pageSize int) ([]Transaction, int64, error) {
	var txs []Transaction
	var total int64

	q := r.db.WithContext(ctx).Model(&Transaction{})

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

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	offset := (page - 1) * pageSize
	err := q.Order("entry_at DESC").Offset(offset).Limit(pageSize).Find(&txs).Error
	return txs, total, errors.FromDB(err, "")
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
