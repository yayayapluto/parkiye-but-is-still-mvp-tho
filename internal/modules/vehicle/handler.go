package vehicle

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

func (h *handler) listVehicleTypes(c *fiber.Ctx) error {
	vts, err := h.svc.ListVehicleTypes(c.Context())
	if err != nil {
		return err
	}
	res := make([]VehicleTypeResponse, 0, len(vts))
	for i := range vts {
		res = append(res, toVehicleTypeResponse(&vts[i]))
	}
	return response.Success(c, "ok", res)
}

func (h *handler) getVehicleType(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid vehicle type id", nil)
	}
	vt, err := h.svc.GetVehicleType(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toVehicleTypeResponse(vt))
}

func (h *handler) createVehicleType(c *fiber.Ctx) error {
	var req CreateVehicleTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}
	vt, err := h.svc.CreateVehicleType(c.Context(), &req)
	if err != nil {
		return err
	}
	return response.Created(c, "vehicle type created", toVehicleTypeResponse(vt))
}

func (h *handler) updateVehicleType(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid vehicle type id", nil)
	}
	var req UpdateVehicleTypeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}
	vt, err := h.svc.UpdateVehicleType(c.Context(), id, &req)
	if err != nil {
		return err
	}
	return response.Success(c, "vehicle type updated", toVehicleTypeResponse(vt))
}

func (h *handler) deleteVehicleType(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid vehicle type id", nil)
	}
	if err := h.svc.DeleteVehicleType(c.Context(), id); err != nil {
		return err
	}
	return response.Success(c, "vehicle type deleted", nil)
}

func (h *handler) listVehicles(c *fiber.Ctx) error {
	var typeID *uuid.UUID
	if raw := c.Query("type_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return response.BadRequest(c, "invalid type_id", nil)
		}
		typeID = &parsed
	}

	pagReq := response.ParsePaginationRequest(c)
	vehicles, total, err := h.svc.ListVehicles(c.Context(), typeID, pagReq.Page, pagReq.PageSize)
	if err != nil {
		return err
	}

	res := make([]VehicleResponse, 0, len(vehicles))
	for i := range vehicles {
		res = append(res, toVehicleResponse(&vehicles[i]))
	}

	queryParams := map[string]string{}
	if typeID != nil {
		queryParams["type_id"] = typeID.String()
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/vehicles",
		pagReq.Page,
		pagReq.PageSize,
		total,
		queryParams,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getVehicle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid vehicle id", nil)
	}
	v, err := h.svc.GetVehicle(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toVehicleResponse(v))
}

func (h *handler) getVehicleByPlate(c *fiber.Ctx) error {
	plate := c.Params("plate")
	if plate == "" {
		return response.BadRequest(c, "plate number is required", nil)
	}
	v, err := h.svc.GetVehicleByPlate(c.Context(), plate)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toVehicleResponse(v))
}

func (h *handler) upsertVehicle(c *fiber.Ctx) error {
	var req UpsertVehicleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}
	_ = middleware.GetUserID(c)
	v, err := h.svc.UpsertVehicle(c.Context(), &req)
	if err != nil {
		return err
	}
	return response.Created(c, "vehicle upserted", toVehicleResponse(v))
}

func (h *handler) updateVehicle(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid vehicle id", nil)
	}
	var req UpdateVehicleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}
	v, err := h.svc.UpdateVehicle(c.Context(), id, &req)
	if err != nil {
		return err
	}
	return response.Success(c, "vehicle updated", toVehicleResponse(v))
}
