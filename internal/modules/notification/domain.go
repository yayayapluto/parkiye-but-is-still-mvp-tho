package notification

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Notification struct {
	ID         uuid.UUID                      `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     *uuid.UUID                     `gorm:"column:user_id;type:uuid;index"`
	RoleTarget *string                        `gorm:"column:role_target;type:varchar(50)"`
	Type       string                         `gorm:"column:type;type:varchar(50);not null"`
	Title      string                         `gorm:"column:title;type:varchar(255);not null"`
	Body       string                         `gorm:"column:body;type:text;not null"`
	Metadata   datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	IsRead     bool           `gorm:"column:is_read;type:boolean;not null;default:false"`
	ReadAt     *time.Time     `gorm:"column:read_at;type:timestamptz"`
	CreatedAt  time.Time      `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (Notification) TableName() string {
	return "notifications"
}
