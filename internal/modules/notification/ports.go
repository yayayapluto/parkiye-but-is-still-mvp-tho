package notification

import (
	"context"

	"github.com/google/uuid"
)

type RepositoryPort interface {
	Create(ctx context.Context, n *Notification) error
	ListForUser(ctx context.Context, userID uuid.UUID, params ListParams) ([]Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
}

type ServicePort interface {
	Notify(ctx context.Context, n Notification) error
	NotifyRole(ctx context.Context, role string, n Notification) error
	ListForUser(ctx context.Context, userID uuid.UUID, params ListParams) ([]Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	Subscribe(userID uuid.UUID) (<-chan Notification, func())
	SubscribeRole(role string) (<-chan Notification, func())
}
