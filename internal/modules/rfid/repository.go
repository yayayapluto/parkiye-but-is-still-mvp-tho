package rfid

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

type rfidCardRepo struct{ db *gorm.DB }

func NewRFIDCardRepository(db *gorm.DB) RFIDCardRepositoryPort {
	return &rfidCardRepo{db}
}

func (r *rfidCardRepo) FindByID(ctx context.Context, id uuid.UUID) (*RFIDCard, error) {
	var card RFIDCard
	err := r.db.WithContext(ctx).First(&card, "id = ?", id).Error
	return &card, errors.FromDB(err, "rfid card not found")
}

func (r *rfidCardRepo) FindByUID(ctx context.Context, cardUID string) (*RFIDCard, error) {
	var card RFIDCard
	err := r.db.WithContext(ctx).Where("card_uid = ?", cardUID).First(&card).Error
	return &card, errors.FromDB(err, "rfid card not found")
}

func (r *rfidCardRepo) FindAll(ctx context.Context, onlyActive bool, page, pageSize int) ([]RFIDCard, int64, error) {
	var cards []RFIDCard
	var total int64

	q := r.db.WithContext(ctx).Model(&RFIDCard{})
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&cards).Error
	return cards, total, errors.FromDB(err, "")
}

func (r *rfidCardRepo) Create(ctx context.Context, card *RFIDCard) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(card).Error, "")
}

func (r *rfidCardRepo) Update(ctx context.Context, card *RFIDCard) error {
	return errors.FromDB(
		r.db.WithContext(ctx).Model(card).Select("*").Updates(card).Error,
		"",
	)
}

func (r *rfidCardRepo) Deactivate(ctx context.Context, id uuid.UUID, deactivatedBy uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&RFIDCard{}).
		Where("id = ? AND is_active = ?", id, true).
		Updates(map[string]any{
			"is_active":      false,
			"deactivated_at": now,
			"deactivated_by": deactivatedBy,
		})
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "rfid card not found or already inactive")
	}
	return nil
}
