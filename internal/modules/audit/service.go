package audit

import (
	"context"

	"github.com/google/uuid"
)

type auditService struct {
	repo AuditLogRepositoryPort
}

func NewService(repo AuditLogRepositoryPort) ServicePort {
	return &auditService{repo: repo}
}

func (s *auditService) ListAuditLogs(ctx context.Context, filter ListFilter, page, pageSize int) ([]AuditLog, int64, error) {
	return s.repo.FindAll(ctx, filter, page, pageSize)
}

func (s *auditService) GetAuditLog(ctx context.Context, id uuid.UUID) (*AuditLog, error) {
	return s.repo.FindByID(ctx, id)
}
