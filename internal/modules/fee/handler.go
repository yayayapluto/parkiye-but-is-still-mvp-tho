package fee

import (
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

func (h *handler) listFeeConfigs(c *fiber.Ctx) error {
	pag := response.ParsePaginationRequest(c)

	var zoneID *uuid.UUID
	if raw := c.Query("zone_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return response.BadRequest(c, "ID zona tidak valid", nil)
		}
		zoneID = &id
	}

	var vehicleTypeID *uuid.UUID
	if raw := c.Query("vehicle_type_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return response.BadRequest(c, "ID jenis kendaraan tidak valid", nil)
		}
		vehicleTypeID = &id
	}

	configs, total, err := h.svc.ListFeeConfigs(c.Context(), zoneID, vehicleTypeID, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	includes := include.ParseInclude(c)
	enrMap := h.svc.EnrichConfigList(c.Context(), configs, includes)

	res := make([]FeeConfigResponse, 0, len(configs))
	for i := range configs {
		var enr *FeeEnrichment
		if enrMap != nil {
			e := enrMap[configs[i].ID]
			enr = &e
		}
		res = append(res, toFeeConfigResponse(&configs[i], enr))
	}

	queryParams := map[string]string{}
	if zoneID != nil {
		queryParams["zone_id"] = zoneID.String()
	}
	if vehicleTypeID != nil {
		queryParams["vehicle_type_id"] = vehicleTypeID.String()
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/fee/configs",
		pag.Page, pag.PageSize, total,
		queryParams,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getFeeConfig(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID konfigurasi tarif tidak valid", nil)
	}
	cfg, err := h.svc.GetFeeConfig(c.Context(), id)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichConfig(c.Context(), cfg, includes)
	return response.Success(c, "ok", toFeeConfigResponse(cfg, enr))
}

func (h *handler) createFeeConfig(c *fiber.Ctx) error {
	var req CreateFeeConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	createdBy := middleware.GetUserID(c)
	cfg, err := h.svc.CreateFeeConfig(c.Context(), req, createdBy)
	if err != nil {
		return err
	}
	return response.Created(c, "fee config created", toFeeConfigResponse(cfg, nil))
}

func (h *handler) deactivateFeeConfig(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID konfigurasi tarif tidak valid", nil)
	}
	if err := h.svc.DeactivateFeeConfig(c.Context(), id); err != nil {
		return err
	}
	return response.Success(c, "fee config deactivated", nil)
}

func (h *handler) listHolidayRates(c *fiber.Ctx) error {
	pag := response.ParsePaginationRequest(c)
	rates, total, err := h.svc.ListHolidayRates(c.Context(), pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	res := make([]HolidayRateResponse, 0, len(rates))
	for i := range rates {
		res = append(res, toHolidayRateResponse(&rates[i]))
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/fee/holiday-rates",
		pag.Page, pag.PageSize, total,
		nil,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getHolidayRate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID tarif libur tidak valid", nil)
	}
	rate, err := h.svc.GetHolidayRate(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toHolidayRateResponse(rate))
}

func (h *handler) createHolidayRate(c *fiber.Ctx) error {
	var req CreateHolidayRateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	createdBy := middleware.GetUserID(c)
	rate, err := h.svc.CreateHolidayRate(c.Context(), req, createdBy)
	if err != nil {
		return err
	}
	return response.Created(c, "holiday rate created", toHolidayRateResponse(rate))
}

func (h *handler) updateHolidayRate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID tarif libur tidak valid", nil)
	}

	var req UpdateHolidayRateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	rate, err := h.svc.UpdateHolidayRate(c.Context(), id, req)
	if err != nil {
		return err
	}
	return response.Success(c, "holiday rate updated", toHolidayRateResponse(rate))
}

func (h *handler) deleteHolidayRate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID tarif libur tidak valid", nil)
	}
	if err := h.svc.DeleteHolidayRate(c.Context(), id); err != nil {
		return err
	}
	return response.Success(c, "holiday rate deleted", nil)
}
