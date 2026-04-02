package override

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"parkieee/pkg/middleware"
	"parkieee/pkg/response"
	"parkieee/pkg/validator"
)

type handler struct {
	svc ServicePort
	v   *validator.Validator
}

func newHandler(svc ServicePort, v *validator.Validator) *handler {
	return &handler{svc: svc, v: v}
}

func (h *handler) listOverrides(c *fiber.Ctx) error {
	operatorID := middleware.GetUserID(c)
	if operatorID == uuid.Nil {
		return response.Unauthorized(c, "Tidak memiliki otorisasi")
	}

	pag := response.ParsePaginationRequest(c)
	overrideType := c.Query("override_type")

	items, total, err := h.svc.List(c.Context(), operatorID, overrideType, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	queryParams := map[string]string{}
	if overrideType != "" {
		queryParams["override_type"] = overrideType
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/overrides",
		pag.Page, pag.PageSize, total,
		queryParams,
	)
	return response.Paginated(c, "ok", items, pagination)
}

func (h *handler) getOverride(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID override tidak valid", nil)
	}

	item, err := h.svc.GetByID(c.Context(), id)
	if err != nil {
		return err
	}

	// Operator hanya boleh lihat override miliknya sendiri.
	operatorID := middleware.GetUserID(c)
	if item.OperatorID != operatorID {
		return response.Forbidden(c, "Akses ditolak")
	}

	return response.Success(c, "ok", item)
}
