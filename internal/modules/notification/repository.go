package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryPort {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *repository) ListForUser(ctx context.Context, userID uuid.UUID, params ListParams) ([]Notification, error) {
	var notifications []Notification
	query := r.db.WithContext(ctx).
		Where("user_id = ? OR role_target IS NOT NULL", userID). // simplifies for now, will refine in service if needed
		Order("created_at DESC")

	if params.UnreadOnly {
		query = query.Where("is_read = ?", false)
	}

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	err := query.Find(&notifications).Error
	return notifications, err
}

func (r *repository) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

func (r *repository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

func (r *repository) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}
