package audit

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

// ListFilter mendefinisikan parameter filter untuk query audit log.
type ListFilter struct {
	ActorID    *uuid.UUID
	EventType  *types.AuditEventType
	TargetType *types.AuditTargetType
	TargetID   *uuid.UUID
	DateFrom   *time.Time
	DateTo     *time.Time
}

// AuditLogResponse adalah representasi audit log yang dikembalikan ke client.
type AuditLogResponse struct {
	ID          uuid.UUID  `json:"id"`
	EventType   string     `json:"event_type"`
	ActorID     uuid.UUID  `json:"actor_id"`
	ActorRole   string     `json:"actor_role"`
	TargetType  string     `json:"target_type"`
	TargetID    *uuid.UUID `json:"target_id,omitempty"`
	IPAddress   string     `json:"ip_address,omitempty"`
	UserAgent   string     `json:"user_agent,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toResponse(l *AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:         l.ID,
		EventType:  string(l.EventType),
		ActorID:    l.ActorID,
		ActorRole:  l.ActorRole,
		TargetType: string(l.TargetType),
		TargetID:   l.TargetID,
		IPAddress:  l.IPAddress,
		UserAgent:  l.UserAgent,
		CreatedAt:  l.CreatedAt,
	}
}
