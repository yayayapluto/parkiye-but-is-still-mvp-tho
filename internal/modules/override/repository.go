package override

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryPort {
	return &repository{db: db}
}

func (r *repository) List(ctx context.Context, operatorID uuid.UUID, overrideType string, page, pageSize int) ([]OperatorOverride, int64, error) {
	var items []OperatorOverride
	var total int64

	q := r.db.WithContext(ctx).Model(&OperatorOverride{}).Where("operator_id = ?", operatorID)
	if overrideType != "" {
		q = q.Where("override_type = ?", overrideType)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*OperatorOverride, error) {
	var item OperatorOverride
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
