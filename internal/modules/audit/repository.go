package audit

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

type auditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepositoryPort {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) FindAll(ctx context.Context, filter ListFilter, page, pageSize int) ([]AuditLog, int64, error) {
	var logs []AuditLog
	var total int64

	q := r.db.WithContext(ctx).Model(&AuditLog{})

	if filter.ActorID != nil {
		q = q.Where("actor_id = ?", *filter.ActorID)
	}
	if filter.EventType != nil {
		q = q.Where("event_type = ?", *filter.EventType)
	}
	if filter.TargetType != nil {
		q = q.Where("target_type = ?", *filter.TargetType)
	}
	if filter.TargetID != nil {
		q = q.Where("target_id = ?", *filter.TargetID)
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

	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, errors.FromDB(err, "")
}

func (r *auditLogRepository) FindByID(ctx context.Context, id uuid.UUID) (*AuditLog, error) {
	var log AuditLog
	err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error
	return &log, errors.FromDB(err, "audit log not found")
}

func (r *auditLogRepository) Append(ctx context.Context, log *AuditLog) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(log).Error, "")
}
