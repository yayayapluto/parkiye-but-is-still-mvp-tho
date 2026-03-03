package payment

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryPort {
	return &repository{db: db}
}

func (r *repository) CreatePayment(ctx context.Context, p *Payment) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError, "failed to create payment")
	}
	return nil
}

func (r *repository) FindPaymentByID(ctx context.Context, id uuid.UUID) (*Payment, error) {
	var p Payment
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.ErrNotFound, "payment not found")
		}
		return nil, errors.Wrap(err, errors.ErrDatabaseError, "failed to find payment")
	}
	return &p, nil
}

func (r *repository) FindPaymentByMidtransOrderID(ctx context.Context, orderID string) (*Payment, error) {
	var p Payment
	if err := r.db.WithContext(ctx).Where("midtrans_order_id = ?", orderID).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.ErrNotFound, "payment not found")
		}
		return nil, errors.Wrap(err, errors.ErrDatabaseError, "failed to find payment by midtrans order id")
	}
	return &p, nil
}

func (r *repository) FindPaymentsByTransactionID(ctx context.Context, txID uuid.UUID) ([]Payment, error) {
	var payments []Payment
	if err := r.db.WithContext(ctx).Where("transaction_id = ?", txID).Order("created_at DESC").Find(&payments).Error; err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError, "failed to find payments by transaction id")
	}
	return payments, nil
}

func (r *repository) UpdatePayment(ctx context.Context, p *Payment) error {
	if err := r.db.WithContext(ctx).Save(p).Error; err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError, "failed to update payment")
	}
	return nil
}

func (r *repository) LogCallback(ctx context.Context, cb *MidtransCallback) error {
	if err := r.db.WithContext(ctx).Create(cb).Error; err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError, "failed to log midtrans callback")
	}
	return nil
}

func (r *repository) UpdateCallback(ctx context.Context, cb *MidtransCallback) error {
	if err := r.db.WithContext(ctx).Save(cb).Error; err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError, "failed to update midtrans callback")
	}
	return nil
}

func (r *repository) CreateRefund(ctx context.Context, ref *Refund) error {
	if err := r.db.WithContext(ctx).Create(ref).Error; err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError, "failed to create refund")
	}
	return nil
}

func (r *repository) FindRefundByID(ctx context.Context, id uuid.UUID) (*Refund, error) {
	var ref Refund
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&ref).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New(errors.ErrNotFound, "refund not found")
		}
		return nil, errors.Wrap(err, errors.ErrDatabaseError, "failed to find refund")
	}
	return &ref, nil
}

func (r *repository) FindRefundByPaymentID(ctx context.Context, paymentID uuid.UUID) ([]Refund, error) {
	var refunds []Refund
	if err := r.db.WithContext(ctx).Where("payment_id = ?", paymentID).Order("created_at DESC").Find(&refunds).Error; err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError, "failed to find refunds by payment id")
	}
	return refunds, nil
}

func (r *repository) ListRefunds(ctx context.Context, page, pageSize int) ([]Refund, int64, error) {
	var refunds []Refund
	var total int64

	offset := (page - 1) * pageSize

	if err := r.db.WithContext(ctx).Model(&Refund{}).Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrDatabaseError, "failed to count refunds")
	}
	if err := r.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&refunds).Error; err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrDatabaseError, "failed to list refunds")
	}
	return refunds, total, nil
}

func (r *repository) UpdateRefund(ctx context.Context, ref *Refund) error {
	if err := r.db.WithContext(ctx).Save(ref).Error; err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError, "failed to update refund")
	}
	return nil
}
