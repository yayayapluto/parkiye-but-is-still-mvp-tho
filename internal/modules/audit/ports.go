package audit

import (
	"context"

	"github.com/google/uuid"
)

// AuditLogRepositoryPort mendefinisikan operasi baca untuk audit log.
// Audit log bersifat INSERT ONLY — tidak ada update/delete dari application layer.
type AuditLogRepositoryPort interface {
	FindAll(ctx context.Context, filter ListFilter, page, pageSize int) ([]AuditLog, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (*AuditLog, error)
	// Append menyimpan satu audit log entry baru.
	Append(ctx context.Context, log *AuditLog) error
}

// ServicePort mendefinisikan kontrak untuk audit service.
type ServicePort interface {
	ListAuditLogs(ctx context.Context, filter ListFilter, page, pageSize int) ([]AuditLog, int64, error)
	GetAuditLog(ctx context.Context, id uuid.UUID) (*AuditLog, error)
}
