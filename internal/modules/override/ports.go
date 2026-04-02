package override

import (
	"context"

	"github.com/google/uuid"
)

type RepositoryPort interface {
	List(ctx context.Context, operatorID uuid.UUID, overrideType string, page, pageSize int) ([]OperatorOverride, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*OperatorOverride, error)
}

type ServicePort interface {
	List(ctx context.Context, operatorID uuid.UUID, overrideType string, page, pageSize int) ([]OverrideResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*OverrideResponse, error)
}
