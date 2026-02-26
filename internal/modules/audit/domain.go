package audit

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"parkieee/pkg/types"
)

// AuditLog is INSERT ONLY — no UPDATE or DELETE at application level.
// Retention: 1 month in DB → export to xlsx/csv → clear.
// Access: engineer only via audit.read permission.
type AuditLog struct {
	ID          uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EventType   types.AuditEventType  `gorm:"type:varchar(50);not null"` // e.g. "fee_config_changed"
	ActorID     uuid.UUID             `gorm:"type:uuid;not null;index"`
	ActorRole   string                `gorm:"type:varchar(50);not null"` // snapshot of actor's role at time of event
	TargetType  types.AuditTargetType `gorm:"type:varchar(50);not null"` // e.g. "fee_config"|"user"|"role"
	TargetID    *uuid.UUID            `gorm:"type:uuid;index"`
	BeforeState datatypes.JSON        `gorm:"type:jsonb"`
	AfterState  datatypes.JSON        `gorm:"type:jsonb"`
	IPAddress   string                `gorm:"type:varchar(45)"`
	UserAgent   string                `gorm:"type:text"`
	CreatedAt   time.Time             `gorm:"not null;default:now()"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// AuditLogExport tracks each monthly export + clear cycle.
type AuditLogExport struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ExportedBy     uuid.UUID          `gorm:"type:uuid;not null"`
	ExportFormat   types.ExportFormat `gorm:"type:varchar(10);not null"` // "xlsx"|"csv"
	FilePath       string             `gorm:"type:text"`
	DateRangeStart time.Time          `gorm:"not null"`
	DateRangeEnd   time.Time          `gorm:"not null"`
	TotalRecords   int                `gorm:"not null;default:0"`
	ExportStatus   types.ExportStatus `gorm:"type:varchar(20);not null"` // "pending"|"completed"|"failed"
	ExportedAt     *time.Time
}

func (AuditLogExport) TableName() string { return "audit_log_exports" }
