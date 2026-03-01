package rfid

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

func (h *handler) listCards(c *fiber.Ctx) error {
	onlyActive := c.QueryBool("active", true)
	pag := response.ParsePaginationRequest(c)

	cards, total, err := h.svc.ListCards(c.Context(), onlyActive, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	res := make([]RFIDCardResponse, 0, len(cards))
	for i := range cards {
		res = append(res, toResponse(&cards[i]))
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/rfid/cards",
		pag.Page, pag.PageSize, total,
		map[string]string{"active": c.Query("active", "true")},
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getCard(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid card id", nil)
	}
	card, err := h.svc.GetCard(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toResponse(card))
}

func (h *handler) getCardByUID(c *fiber.Ctx) error {
	uid := c.Params("uid")
	if uid == "" {
		return response.BadRequest(c, "card uid is required", nil)
	}
	card, err := h.svc.GetCardByUID(c.Context(), uid)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toResponse(card))
}

func (h *handler) registerOrGet(c *fiber.Ctx) error {
	var req RegisterOrGetRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	card, err := h.svc.RegisterOrGet(c.Context(), req.CardUID)
	if err != nil {
		return err
	}
	return response.Created(c, "rfid card registered", toResponse(card))
}

func (h *handler) linkVehicle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid card id", nil)
	}

	var req LinkVehicleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	card, err := h.svc.LinkVehicle(c.Context(), id, req.VehicleID)
	if err != nil {
		return err
	}
	return response.Success(c, "vehicle linked", toResponse(card))
}

func (h *handler) deactivate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid card id", nil)
	}

	operatorID := middleware.GetUserID(c)

	if err := h.svc.Deactivate(c.Context(), id, operatorID); err != nil {
		return err
	}
	return response.Success(c, "rfid card deactivated", nil)
}
