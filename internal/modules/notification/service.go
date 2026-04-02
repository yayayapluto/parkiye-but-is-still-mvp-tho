package notification

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"parkieee/pkg/logger"
)

type service struct {
	repo     RepositoryPort
	userSubs map[uuid.UUID][]chan Notification
	roleSubs map[string][]chan Notification
	mu       sync.RWMutex
	log      logger.Logger
}

func NewService(repo RepositoryPort, log logger.Logger) ServicePort {
	return &service{
		repo:     repo,
		userSubs: make(map[uuid.UUID][]chan Notification),
		roleSubs: make(map[string][]chan Notification),
		log:      log.With("module", "notification"),
	}
}

func (s *service) Notify(ctx context.Context, n Notification) error {
	if err := s.repo.Create(ctx, &n); err != nil {
		s.log.Error(ctx, "failed to create notification", "error", err)
		return err
	}

	if n.UserID != nil {
		s.mu.RLock()
		subs := s.userSubs[*n.UserID]
		s.mu.RUnlock()

		for _, ch := range subs {
			select {
			case ch <- n:
			default:
				s.log.Warn(ctx, "dropping personal notification: subscriber too slow", "user_id", *n.UserID)
			}
		}
	}

	return nil
}

func (s *service) NotifyRole(ctx context.Context, role string, n Notification) error {
	n.RoleTarget = &role
	if err := s.repo.Create(ctx, &n); err != nil {
		s.log.Error(ctx, "failed to create role notification", "error", err)
		return err
	}

	s.mu.RLock()
	subs := s.roleSubs[role]
	s.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- n:
		default:
			s.log.Warn(ctx, "dropping role notification: subscriber too slow", "role", role)
		}
	}

	return nil
}

func (s *service) ListForUser(ctx context.Context, userID uuid.UUID, params ListParams) ([]Notification, error) {
	return s.repo.ListForUser(ctx, userID, params)
}

func (s *service) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return s.repo.MarkRead(ctx, id, userID)
}

func (s *service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *service) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *service) Subscribe(userID uuid.UUID) (<-chan Notification, func()) {
	ch := make(chan Notification, 10)
	s.mu.Lock()
	s.userSubs[userID] = append(s.userSubs[userID], ch)
	s.mu.Unlock()

	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subs := s.userSubs[userID]
		for i, sub := range subs {
			if sub == ch {
				s.userSubs[userID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, cleanup
}

func (s *service) SubscribeRole(role string) (<-chan Notification, func()) {
	ch := make(chan Notification, 10)
	s.mu.Lock()
	s.roleSubs[role] = append(s.roleSubs[role], ch)
	s.mu.Unlock()

	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subs := s.roleSubs[role]
		for i, sub := range subs {
			if sub == ch {
				s.roleSubs[role] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, cleanup
}
