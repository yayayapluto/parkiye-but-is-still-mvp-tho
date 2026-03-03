package transaction

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"parkieee/pkg/config"
	"parkieee/pkg/middleware"
	"parkieee/pkg/photo"
	"parkieee/pkg/response"
	"parkieee/pkg/types"
	"parkieee/pkg/validator"
)

type handler struct {
	svc   ServicePort
	v     *validator.Validator
	s3cfg config.S3Config
}

func newHandler(svc ServicePort, v *validator.Validator, s3cfg config.S3Config) *handler {
	return &handler{svc: svc, v: v, s3cfg: s3cfg}
}

func (h *handler) listTransactions(c *fiber.Ctx) error {
	pag := response.ParsePaginationRequest(c)

	filter := ListFilter{}

	if s := c.Query("status"); s != "" {
		status := types.TransactionStatus(s)
		filter.Status = &status
	}
	if s := c.Query("zone_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return response.BadRequest(c, "invalid zone_id", nil)
		}
		filter.ZoneID = &id
	}
	if s := c.Query("entry_gate_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return response.BadRequest(c, "invalid entry_gate_id", nil)
		}
		filter.EntryGateID = &id
	}
	if s := c.Query("entry_method"); s != "" {
		m := types.EntryMethod(s)
		filter.EntryMethod = &m
	}
	if s := c.Query("date_from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return response.BadRequest(c, "invalid date_from, use YYYY-MM-DD", nil)
		}
		filter.DateFrom = &t
	}
	if s := c.Query("date_to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return response.BadRequest(c, "invalid date_to, use YYYY-MM-DD", nil)
		}
		// Include the entire day.
		endOfDay := t.Add(24*time.Hour - time.Second)
		filter.DateTo = &endOfDay
	}

	txs, total, err := h.svc.ListTransactions(c.Context(), filter, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	res := make([]TransactionResponse, 0, len(txs))
	for i := range txs {
		res = append(res, toResponse(&txs[i], nil))
	}

	queryParams := map[string]string{}
	if filter.Status != nil {
		queryParams["status"] = string(*filter.Status)
	}
	if filter.ZoneID != nil {
		queryParams["zone_id"] = filter.ZoneID.String()
	}

	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/transactions",
		pag.Page, pag.PageSize, total,
		queryParams,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) getTransaction(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid transaction id", nil)
	}
	tx, err := h.svc.GetTransaction(c.Context(), id)
	if err != nil {
		return err
	}
	ocr := h.svc.LoadOCRSummary(c.Context(), tx.ID)
	return response.Success(c, "ok", toResponse(tx, ocr))
}

func (h *handler) getByCode(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.BadRequest(c, "transaction code is required", nil)
	}
	tx, err := h.svc.GetByCode(c.Context(), code)
	if err != nil {
		return err
	}
	ocr := h.svc.LoadOCRSummary(c.Context(), tx.ID)
	return response.Success(c, "ok", toResponse(tx, ocr))
}

func (h *handler) getLogs(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid transaction id", nil)
	}
	logs, err := h.svc.GetLogs(c.Context(), id)
	if err != nil {
		return err
	}
	res := make([]TransactionLogResponse, 0, len(logs))
	for i := range logs {
		res = append(res, toLogResponse(&logs[i]))
	}
	return response.Success(c, "ok", res)
}

func (h *handler) recordEntry(c *fiber.Ctx) error {
	var req RecordEntryRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	if form, err := c.MultipartForm(); err == nil {
		if files := form.File["photo"]; len(files) > 0 {
			publicURL, volumePath, photoErr := photo.Save(files[0], "entry", h.s3cfg)
			if photoErr != nil {
				return fmt.Errorf("save entry photo: %w", photoErr)
			}
			req.EntryPhotoURL = publicURL
			req.EntryPhotoPath = volumePath
		}
	}

	operatorID := middleware.GetUserID(c)

	tx, err := h.svc.RecordEntry(c.Context(), req, operatorID)
	if err != nil {
		return err
	}
	return response.Created(c, fmt.Sprintf("entry recorded: %s", tx.TransactionCode), toResponse(tx, nil))
}

func (h *handler) recordExit(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid transaction id", nil)
	}

	var req RecordExitRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	if form, err := c.MultipartForm(); err == nil {
		if files := form.File["photo"]; len(files) > 0 {
			publicURL, volumePath, photoErr := photo.Save(files[0], "exit", h.s3cfg)
			if photoErr != nil {
				return fmt.Errorf("save exit photo: %w", photoErr)
			}
			req.ExitPhotoURL = publicURL
			req.ExitPhotoPath = volumePath
		}
	}

	operatorID := middleware.GetUserID(c)

	tx, err := h.svc.RecordExit(c.Context(), id, req, operatorID)
	if err != nil {
		return err
	}
	return response.Success(c, fmt.Sprintf("exit recorded, fee: Rp%d", *tx.CalculatedFee), toResponse(tx, nil))
}

func (h *handler) cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid transaction id", nil)
	}

	var req CancelRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	operatorID := middleware.GetUserID(c)

	tx, err := h.svc.Cancel(c.Context(), id, req.Reason, operatorID)
	if err != nil {
		return err
	}
	return response.Success(c, "transaction cancelled", toResponse(tx, nil))
}
