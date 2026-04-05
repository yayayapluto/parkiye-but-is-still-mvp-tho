package audit

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"parkieee/pkg/response"
	"parkieee/pkg/types"
)

type handler struct {
	svc ServicePort
}

func newHandler(svc ServicePort) *handler {
	return &handler{svc: svc}
}

// listAuditLogs handles GET /audit-logs
func (h *handler) listAuditLogs(c *fiber.Ctx) error {
	pagReq := response.ParsePaginationRequest(c)

	filter := ListFilter{}

	if s := c.Query("actor_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return response.BadRequest(c, "ID aktor tidak valid", nil)
		}
		filter.ActorID = &id
	}
	if s := c.Query("event_type"); s != "" {
		et := types.AuditEventType(s)
		filter.EventType = &et
	}
	if s := c.Query("target_type"); s != "" {
		tt := types.AuditTargetType(s)
		filter.TargetType = &tt
	}
	if s := c.Query("target_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return response.BadRequest(c, "ID target tidak valid", nil)
		}
		filter.TargetID = &id
	}
	if s := c.Query("date_from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return response.BadRequest(c, "Format tanggal awal tidak valid, gunakan YYYY-MM-DD", nil)
		}
		filter.DateFrom = &t
	}
	if s := c.Query("date_to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return response.BadRequest(c, "Format tanggal akhir tidak valid, gunakan YYYY-MM-DD", nil)
		}
		end := t.Add(24*time.Hour - time.Second)
		filter.DateTo = &end
	}

	logs, total, err := h.svc.ListAuditLogs(c.Context(), filter, pagReq.Page, pagReq.PageSize)
	if err != nil {
		return err
	}

	res := make([]AuditLogResponse, 0, len(logs))
	for i := range logs {
		res = append(res, toResponse(&logs[i]))
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/audit-logs",
		pagReq.Page, pagReq.PageSize, total,
		map[string]string{},
		nil,
	)

	return response.Paginated(c, "ok", res, pagination)
}

// getAuditLog handles GET /audit-logs/:id
func (h *handler) getAuditLog(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID audit log tidak valid", nil)
	}

	log, err := h.svc.GetAuditLog(c.Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", toResponse(log))
}
