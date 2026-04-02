package override

import (
	"context"

	"github.com/google/uuid"
	"parkieee/pkg/errors"
)

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) ServicePort {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, operatorID uuid.UUID, overrideType string, page, pageSize int) ([]OverrideResponse, int64, error) {
	items, total, err := s.repo.List(ctx, operatorID, overrideType, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	result := make([]OverrideResponse, len(items))
	for i := range items {
		result[i] = toResponse(&items[i])
	}
	return result, total, nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*OverrideResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New(errors.ErrNotFound, "Override tidak ditemukan")
	}
	res := toResponse(item)
	return &res, nil
}
