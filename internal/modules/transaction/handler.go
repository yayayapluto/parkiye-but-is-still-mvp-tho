package transaction

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"parkieee/pkg/config"
	"parkieee/pkg/include"
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

	filter := ListFilter{
		SortBy:    pag.SortBy,
		SortOrder: pag.SortOrder,
		Search:    c.Query("search"),
	}

	if s := c.Query("status"); s != "" {
		status := types.TransactionStatus(s)
		filter.Status = &status
	}
	if s := c.Query("zone_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return response.BadRequest(c, "ID zona tidak valid", nil)
		}
		filter.ZoneID = &id
	}
	if s := c.Query("entry_gate_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return response.BadRequest(c, "ID gate masuk tidak valid", nil)
		}
		filter.EntryGateID = &id
	}
	if s := c.Query("entry_method"); s != "" {
		m := types.EntryMethod(s)
		filter.EntryMethod = &m
	}

	if s := c.Query("plate_mismatch"); s != "" {
		val := s == "true"
		filter.PlateMismatch = &val
	}

	if s := c.Query("is_unclosed"); s != "" {
		val := s == "true"
		filter.IsUnclosed = &val
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

	txs, total, err := h.svc.ListTransactions(c.Context(), filter, pag.Page, pag.PageSize)
	if err != nil {
		return err
	}

	includes := include.ParseInclude(c)
	enrMap := h.svc.EnrichTransactionList(c.Context(), txs, includes)

	res := make([]TransactionResponse, 0, len(txs))
	for i := range txs {
		var enr *TransactionEnrichment
		if enrMap != nil {
			e := enrMap[txs[i].ID]
			enr = &e
		}
		res = append(res, toResponse(&txs[i], nil, enr))
	}

	// Prepare queryParams for reliable pagination links
	queryParams := map[string]string{
		"sort_by":    filter.SortBy,
		"sort_order": filter.SortOrder,
		"search":     filter.Search,
	}
	if filter.Status != nil {
		queryParams["status"] = string(*filter.Status)
	}
	if filter.ZoneID != nil {
		queryParams["zone_id"] = filter.ZoneID.String()
	}
	if filter.PlateMismatch != nil {
		if *filter.PlateMismatch {
			queryParams["plate_mismatch"] = "true"
		} else {
			queryParams["plate_mismatch"] = "false"
		}
	}
	if filter.IsUnclosed != nil {
		if *filter.IsUnclosed {
			queryParams["is_unclosed"] = "true"
		} else {
			queryParams["is_unclosed"] = "false"
		}
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
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}
	tx, err := h.svc.GetTransaction(c.Context(), id)
	if err != nil {
		return err
	}
	ocr := h.svc.LoadOCRSummary(c.Context(), tx.ID)
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Success(c, "ok", toResponse(tx, ocr, enr))
}

func (h *handler) getByCode(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return response.BadRequest(c, "Kode transaksi wajib diisi", nil)
	}
	tx, err := h.svc.GetByCode(c.Context(), code)
	if err != nil {
		return err
	}
	ocr := h.svc.LoadOCRSummary(c.Context(), tx.ID)
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Success(c, "ok", toResponse(tx, ocr, enr))
}

func (h *handler) getLogs(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
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
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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

	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Created(c, fmt.Sprintf("entry recorded: %s", tx.TransactionCode), toResponse(tx, nil, enr))
}

func (h *handler) getOpenByRFID(c *fiber.Ctx) error {
	uid := c.Params("uid")
	if uid == "" {
		return response.BadRequest(c, "UID RFID wajib diisi", nil)
	}
	tx, err := h.svc.GetOpenByRFIDUID(c.Context(), uid)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Success(c, "transaction found", toResponse(tx, nil, enr))
}

func (h *handler) recordExit(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}

	var req RecordExitRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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

	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Success(c, fmt.Sprintf("exit recorded, fee: Rp%d", *tx.CalculatedFee), toResponse(tx, nil, enr))
}

func (h *handler) simulate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}
	var body struct {
		MinutesAgo int `json:"minutes_ago"`
	}
	if err := c.BodyParser(&body); err != nil || body.MinutesAgo <= 0 {
		return response.BadRequest(c, "Nilai minutes_ago harus bilangan bulat positif", nil)
	}
	tx, err := h.svc.SimulateEntryTime(c.Context(), id, body.MinutesAgo)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Success(c, "entry_at simulated", toResponse(tx, nil, enr))
}

func (h *handler) cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}

	var req CancelRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	operatorID := middleware.GetUserID(c)

	tx, err := h.svc.Cancel(c.Context(), id, req.Reason, operatorID)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichTransaction(c.Context(), tx, includes)

	return response.Success(c, "transaction cancelled", toResponse(tx, nil, enr))
}
