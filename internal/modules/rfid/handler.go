package rfid

import (
	"time"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"parkieee/pkg/include"
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

func (h *handler) listCards(c *fiber.Ctx) error {
	filter := ListRFIDFilter{
		Search:    c.Query("search"),
		CardUID:   c.Query("card_uid"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	if s := c.Query("active"); s != "" {
		val := s == "true"
		filter.IsActive = &val
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
		// Include the entire day.
		endOfDay := t.Add(24*time.Hour - time.Second)
		filter.DateTo = &endOfDay
	}

	pag := response.ParsePaginationRequest(c)
	cards, total, err := h.svc.ListCards(c.Context(), filter, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	includes := include.ParseInclude(c)
	enrMap := h.svc.EnrichCardList(c.Context(), cards, includes)

	res := make([]RFIDCardResponse, 0, len(cards))
	for i := range cards {
		var enr *RFIDCardEnrichment
		if enrMap != nil {
			e := enrMap[cards[i].ID]
			enr = &e
		}
		res = append(res, toResponse(&cards[i], enr))
	}

	queryParams := map[string]string{
		"search":     filter.Search,
		"card_uid":   filter.CardUID,
		"sort_by":    filter.SortBy,
		"sort_order": filter.SortOrder,
	}
	if filter.IsActive != nil {
		if *filter.IsActive {
			queryParams["active"] = "true"
		} else {
			queryParams["active"] = "false"
		}
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/rfid/cards",
		pag.Page, pag.PageSize, total,
		queryParams,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getCard(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID kartu tidak valid", nil)
	}
	card, err := h.svc.GetCard(c.Context(), id)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichCard(c.Context(), card, includes)
	return response.Success(c, "ok", toResponse(card, enr))
}

func (h *handler) getCardByUID(c *fiber.Ctx) error {
	uid := c.Params("uid")
	if uid == "" {
		return response.BadRequest(c, "UID kartu wajib diisi", nil)
	}
	card, err := h.svc.GetCardByUID(c.Context(), uid)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichCard(c.Context(), card, includes)
	return response.Success(c, "ok", toResponse(card, enr))
}


func (h *handler) registerOrGet(c *fiber.Ctx) error {
	var req RegisterOrGetRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	card, err := h.svc.RegisterOrGet(c.Context(), req.CardUID)
	if err != nil {
		return err
	}
	return response.Created(c, "rfid card registered", toResponse(card, nil))
}

func (h *handler) linkVehicle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID kartu tidak valid", nil)
	}

	var req LinkVehicleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	card, err := h.svc.LinkVehicle(c.Context(), id, req.VehicleID)
	if err != nil {
		return err
	}
	return response.Success(c, "vehicle linked", toResponse(card, nil))
}

func (h *handler) deactivate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID kartu tidak valid", nil)
	}

	operatorID := middleware.GetUserID(c)

	if err := h.svc.Deactivate(c.Context(), id, operatorID); err != nil {
		return err
	}
	return response.Success(c, "rfid card deactivated", nil)
}
