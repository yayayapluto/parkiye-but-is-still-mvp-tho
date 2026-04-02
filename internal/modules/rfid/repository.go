package rfid

import (
	"context"
	"fmt"
	"strings"
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

func (r *rfidCardRepo) FindAll(ctx context.Context, filter ListRFIDFilter, page, pageSize int) ([]RFIDCard, int64, error) {
	var cards []RFIDCard
	var total int64

	q := r.db.WithContext(ctx).Model(&RFIDCard{})
	if filter.IsActive != nil {
		q = q.Where("is_active = ?", *filter.IsActive)
	}

	if filter.CardUID != "" {
		q = q.Where("card_uid ILIKE ?", "%"+filter.CardUID+"%")
	}

	if filter.Search != "" {
		q = q.Where("card_uid ILIKE ?", "%"+filter.Search+"%")
	}

	if filter.DateFrom != nil {
		q = q.Where("created_at >= ?", *filter.DateFrom)
	}

	if filter.DateTo != nil {
		q = q.Where("created_at <= ?", *filter.DateTo)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	sortCol := "created_at"
	sortOrder := "DESC"

	if filter.SortBy != "" {
		sortCol = filter.SortBy
	}
	if strings.ToLower(filter.SortOrder) == "asc" {
		sortOrder = "ASC"
	}

	offset := (page - 1) * pageSize
	err := q.Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&cards).Error
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
