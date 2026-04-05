package zone

import (
	"fmt"

	"parkieee/pkg/middleware"
	"parkieee/pkg/response"
	"parkieee/pkg/validator"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type handler struct {
	svc ServicePort
	v   *validator.Validator
}

func newHandler(svc ServicePort, v *validator.Validator) *handler {
	return &handler{svc: svc, v: v}
}

func (h *handler) listZones(c *fiber.Ctx) error {
	pagReq := response.ParsePaginationRequest(c)

	filter := ListZoneFilter{
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
		Active:    c.QueryBool("active", true),
	}

	zones, total, err := h.svc.ListZones(c.Context(), filter, pagReq.Page, pagReq.PageSize)
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
		pagReq.Page, pagReq.PageSize, total,
		map[string]string{
			"active":     c.Query("active", "true"),
			"search":     filter.Search,
			"sort_by":    filter.SortBy,
			"sort_order": filter.SortOrder,
		},
		nil,
	)

	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getZone(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID zona tidak valid", nil)
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
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		return response.BadRequest(c, "ID zona tidak valid", nil)
	}

	var req UpdateZoneRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		return response.BadRequest(c, "ID zona tidak valid", nil)
	}

	if err := h.svc.DeactivateZone(c.Context(), id); err != nil {
		return err
	}

	return response.Success(c, "zone deactivated", nil)
}

// listAllCapacities handles GET /zones/capacity
// Returns the latest capacity snapshot for ALL active zones in one call.
func (h *handler) listAllCapacities(c *fiber.Ctx) error {
	capacities, err := h.svc.ListAllCapacities(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", capacities)
}

func (h *handler) getCapacity(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID zona tidak valid", nil)
	}

	capacity, err := h.svc.GetCapacity(c.Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", capacity)
}

func (h *handler) listAllGates(c *fiber.Ctx) error {
	pagReq := response.ParsePaginationRequest(c)

	filter := ListGateFilter{
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
		Active:    c.QueryBool("active", false),
	}

	if z := c.Query("zone_id"); z != "" {
		parsed, err := uuid.Parse(z)
		if err != nil {
			return response.BadRequest(c, "ID zona tidak valid", nil)
		}
		filter.ZoneID = &parsed
	}

	if t := c.Query("gate_type"); t == "entry" || t == "exit" {
		filter.GateType = &t
	}

	gates, total, err := h.svc.ListAllGates(c.Context(), filter, pagReq.Page, pagReq.PageSize)
	if err != nil {
		return err
	}

	res := make([]GateResponse, 0, len(gates))
	for i := range gates {
		res = append(res, toGateResponse(&gates[i]))
	}

	extraParams := map[string]string{
		"active":     c.Query("active", "false"),
		"search":     filter.Search,
		"sort_by":    filter.SortBy,
		"sort_order": filter.SortOrder,
	}
	if filter.ZoneID != nil {
		extraParams["zone_id"] = filter.ZoneID.String()
	}
	if filter.GateType != nil {
		extraParams["gate_type"] = *filter.GateType
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/gates",
		pagReq.Page, pagReq.PageSize, total,
		extraParams,
		nil,
	)

	return response.Paginated(c, "ok", res, pagination)
}
func (h *handler) listGates(c *fiber.Ctx) error {
	zoneID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID zona tidak valid", nil)
	}

	pagReq := response.ParsePaginationRequest(c)

	filter := ListGateFilter{
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
		Active:    c.QueryBool("active", true),
		ZoneID:    &zoneID,
	}

	gates, total, err := h.svc.ListGates(c.Context(), filter, pagReq.Page, pagReq.PageSize)
	if err != nil {
		return err
	}

	res := make([]GateResponse, 0, len(gates))
	for i := range gates {
		res = append(res, toGateResponse(&gates[i]))
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		fmt.Sprintf("/api/v1/zones/%s/gates", zoneID),
		pagReq.Page, pagReq.PageSize, total,
		map[string]string{
			"active":     c.Query("active", "true"),
			"search":     filter.Search,
			"sort_by":    filter.SortBy,
			"sort_order": filter.SortOrder,
		},
		nil,
	)

	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getGate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "ID gate tidak valid", nil)
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
		return response.BadRequest(c, "ID zona tidak valid", nil)
	}

	var req CreateGateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}

	req.ZoneID = zoneID

	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	var req UpdateGateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	if err := h.svc.DeactivateGate(c.Context(), id); err != nil {
		return err
	}

	return response.Success(c, "gate deactivated", nil)
}

func (h *handler) regenerateGateToken(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	gate, err := h.svc.RegenerateGateToken(c.Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, "gate token regenerated", toGateResponse(gate))
}

func (h *handler) updateGateMode(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	var req UpdateGateModeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	actorID := middleware.GetUserID(c)
	gate, err := h.svc.UpdateGateMode(c.Context(), id, req.Mode, actorID)
	if err != nil {
		return err
	}

	return response.Success(c, "gate mode updated", toGateResponse(gate))
}

func (h *handler) assignCashier(c *fiber.Ctx) error {
	gateID, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	var req AssignCashierRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	actorID := middleware.GetUserID(c)
	a, err := h.svc.AssignCashier(c.Context(), gateID, req.UserID, actorID)
	if err != nil {
		return err
	}

	return response.Success(c, "cashier assigned", toAssignmentResponse(a))
}

func (h *handler) unassignCashier(c *fiber.Ctx) error {
	gateID, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	if err := h.svc.UnassignCashier(c.Context(), gateID); err != nil {
		return err
	}

	return response.Success(c, "cashier unassigned", nil)
}

func (h *handler) getCashierAssignment(c *fiber.Ctx) error {
	gateID, err := uuid.Parse(c.Params("gateId"))
	if err != nil {
		return response.BadRequest(c, "ID gate tidak valid", nil)
	}

	a, err := h.svc.GetCashierAssignment(c.Context(), gateID)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", toAssignmentResponse(a))
}
