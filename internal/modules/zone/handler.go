package zone

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

func (h *handler) listZones(c *fiber.Ctx) error {
	onlyActive := c.QueryBool("active", true)
	pagReq := response.ParsePaginationRequest(c)

	zones, total, err := h.svc.ListZones(c.Context(), onlyActive, pagReq.Page, pagReq.PageSize)
	if err != nil {
		return err
	}

	res := make([]ZoneResponse, 0, len(zones))
	for i := range zones {
		res = append(res, toZoneResponse(&zones[i]))
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/zones",
		pagReq.Page,
		pagReq.PageSize,
		total,
		map[string]string{"active": c.Query("active", "true")},
	)

	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getZone(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid zone id", nil)
	}

	zone, err := h.svc.GetZone(c.Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", toZoneResponse(zone))
}

func (h *handler) createZone(c *fiber.Ctx) error {
	var req CreateZoneRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	actorID := middleware.GetUserID(c)

	zone, err := h.svc.CreateZone(c.Context(), &req, actorID)
	if err != nil {
		return err
	}

	return response.Created(c, "zone created", toZoneResponse(zone))
}

func (h *handler) updateZone(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid zone id", nil)
	}

	var req UpdateZoneRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	zone, err := h.svc.UpdateZone(c.Context(), id, &req)
	if err != nil {
		return err
	}

	return response.Success(c, "zone updated", toZoneResponse(zone))
}

func (h *handler) deactivateZone(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid zone id", nil)
	}

	if err := h.svc.DeactivateZone(c.Context(), id); err != nil {
		return err
	}

	return response.Success(c, "zone deactivated", nil)
}

func (h *handler) getCapacity(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid zone id", nil)
	}

	capacity, err := h.svc.GetCapacity(c.Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", capacity)
}

func (h *handler) listGates(c *fiber.Ctx) error {
	zoneID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid zone id", nil)
	}

	onlyActive := c.QueryBool("active", true)

	gates, err := h.svc.ListGates(c.Context(), zoneID, onlyActive)
	if err != nil {
		return err
	}

	res := make([]GateResponse, 0, len(gates))
	for i := range gates {
		res = append(res, toGateResponse(&gates[i]))
	}

	return response.Success(c, "ok", res)
}

func (h *handler) getGate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "invalid gate id", nil)
	}

	gate, err := h.svc.GetGate(c.Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", toGateResponse(gate))
}

func (h *handler) createGate(c *fiber.Ctx) error {
	zoneID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid zone id", nil)
	}

	var req CreateGateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}

	req.ZoneID = zoneID

	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	actorID := middleware.GetUserID(c)

	gate, err := h.svc.CreateGate(c.Context(), &req, actorID)
	if err != nil {
		return err
	}

	return response.Created(c, "gate created", toGateResponse(gate))
}

func (h *handler) updateGate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "invalid gate id", nil)
	}

	var req UpdateGateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	gate, err := h.svc.UpdateGate(c.Context(), id, &req)
	if err != nil {
		return err
	}

	return response.Success(c, "gate updated", toGateResponse(gate))
}

func (h *handler) deactivateGate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "invalid gate id", nil)
	}

	if err := h.svc.DeactivateGate(c.Context(), id); err != nil {
		return err
	}

	return response.Success(c, "gate deactivated", nil)
}
