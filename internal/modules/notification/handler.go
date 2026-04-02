package notification

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"parkieee/pkg/middleware"
	"parkieee/pkg/response"
)

type handler struct {
	svc ServicePort
}

func newHandler(svc ServicePort) *handler {
	return &handler{svc: svc}
}

func (h *handler) listForUser(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return response.Unauthorized(c, "Akses ditolak: user tidak ditemukan")
	}

	params := ListParams{
		UnreadOnly: c.Query("unread_only") == "true",
		Limit:      c.QueryInt("limit", 20),
		Offset:     c.QueryInt("offset", 0),
	}

	notifications, err := h.svc.ListForUser(c.Context(), userID, params)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", toResponseList(notifications))
}

func (h *handler) getUnreadCount(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return response.Unauthorized(c, "Akses ditolak: user tidak ditemukan")
	}

	count, err := h.svc.CountUnread(c.Context(), userID)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", fiber.Map{"count": count})
}

func (h *handler) markRead(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID notifikasi tidak valid", nil)
	}

	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return response.Unauthorized(c, "Akses ditolak: user tidak ditemukan")
	}

	if err := h.svc.MarkRead(c.Context(), id, userID); err != nil {
		return err
	}

	return response.Success(c, "ok", nil)
}

func (h *handler) markAllRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return response.Unauthorized(c, "Akses ditolak: user tidak ditemukan")
	}

	if err := h.svc.MarkAllRead(c.Context(), userID); err != nil {
		return err
	}

	return response.Success(c, "ok", nil)
}

func (h *handler) stream(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return response.Unauthorized(c, "Akses ditolak: user tidak ditemukan")
	}

	// Also subscribe to specific roles if user has them
	// In this simplified version, we'll just check if they are operator/cashier via role if available,
	// but the requirement says "Subscribe(userID)" and "SubscribeRole(role)".
	// We'll subscribe the user to their personal channel AND their role channels.

	userRole := middleware.GetUserRole(c)

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(bw *bufio.Writer) {

		userCh, userCleanup := h.svc.Subscribe(userID)
		defer userCleanup()

		var roleCh <-chan Notification
		var roleCleanup func()
		if userRole != "" {
			roleCh, roleCleanup = h.svc.SubscribeRole(string(userRole))
			defer roleCleanup()
		}

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		fmt.Fprintf(bw, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
		bw.Flush()

		for {
			select {
			case n, ok := <-userCh:
				if !ok {
					return
				}
				h.writeNotification(bw, n)
			case n, ok := <-roleCh:
				if !ok {
					return
				}
				if n.UserID != nil && *n.UserID == userID {
					// skip if already sent via personal channel (redundancy check)
					continue
				}
				h.writeNotification(bw, n)
			case <-ticker.C:
				fmt.Fprintf(bw, "event: heartbeat\ndata: {\"time\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
				bw.Flush()
			}
		}
	}))

	return nil
}

func (h *handler) writeNotification(bw *bufio.Writer, n Notification) {
	data, _ := json.Marshal(toResponse(n))
	fmt.Fprintf(bw, "event: notification\ndata: %s\n\n", data)
	bw.Flush()
}
