package notification

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type NotificationResponse struct {
	ID         uuid.UUID         `json:"id"`
	UserID     *uuid.UUID        `json:"user_id"`
	RoleTarget *string           `json:"role_target"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Metadata   map[string]any `json:"metadata"`
	IsRead     bool              `json:"is_read"`
	ReadAt     *time.Time        `json:"read_at"`
	CreatedAt  time.Time         `json:"created_at"`
}

type ListParams struct {
	UnreadOnly bool
	Limit      int
	Offset     int
}

func toResponse(n Notification) NotificationResponse {
	var metadata map[string]any
	if len(n.Metadata) > 0 {
		_ = json.Unmarshal(n.Metadata, &metadata)
	}
	return NotificationResponse{
		ID:         n.ID,
		UserID:     n.UserID,
		RoleTarget: n.RoleTarget,
		Type:       n.Type,
		Title:      n.Title,
		Body:       n.Body,
		Metadata:   metadata,
		IsRead:     n.IsRead,
		ReadAt:     n.ReadAt,
		CreatedAt:  n.CreatedAt,
	}
}

func toResponseList(notifications []Notification) []NotificationResponse {
	res := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		res[i] = toResponse(n)
	}
	return res
}
