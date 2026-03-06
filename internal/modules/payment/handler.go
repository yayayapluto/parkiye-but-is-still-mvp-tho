package payment

import (
	"encoding/json"

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

func (h *handler) payCash(c *fiber.Ctx) error {
	var req PayCashRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	handledBy := middleware.GetUserID(c)
	p, err := h.svc.PayCash(c.Context(), req, handledBy)
	if err != nil {
		return err
	}
	return response.Created(c, "cash payment processed", toPaymentResponse(p))
}

func (h *handler) initiateQRIS(c *fiber.Ctx) error {
	var req InitiateQRISRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	p, err := h.svc.InitiateQRIS(c.Context(), req)
	if err != nil {
		return err
	}
	return response.Created(c, "QRIS payment initiated", toPaymentResponse(p))
}

func (h *handler) webhookMidtrans(c *fiber.Ctx) error {
	rawBody := c.Body()

	var payload MidtransWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		// Always 200 to Midtrans — don't retry on parse error
		return response.Success(c, "ok", nil)
	}

	// Fire and forget — always return 200
	_ = h.svc.HandleMidtransWebhook(c.Context(), rawBody, payload)
	return response.Success(c, "ok", nil)
}

func (h *handler) getPayment(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid payment id", nil)
	}
	p, err := h.svc.GetPayment(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toPaymentResponse(p))
}

func (h *handler) pollPaymentStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid payment id", nil)
	}
	p, err := h.svc.PollPaymentStatus(c.Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", toPaymentResponse(p))
}

func (h *handler) listByTransaction(c *fiber.Ctx) error {
	txID, err := uuid.Parse(c.Params("txID"))
	if err != nil {
		return response.BadRequest(c, "invalid transaction id", nil)
	}
	payments, err := h.svc.ListByTransaction(c.Context(), txID)
	if err != nil {
		return err
	}
	res := make([]PaymentResponse, 0, len(payments))
	for i := range payments {
		res = append(res, toPaymentResponse(&payments[i]))
	}
	return response.Success(c, "ok", res)
}

func (h *handler) requestRefund(c *fiber.Ctx) error {
	var req RequestRefundRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	requestedBy := middleware.GetUserID(c)
	ref, err := h.svc.RequestRefund(c.Context(), req, requestedBy)
	if err != nil {
		return err
	}
	return response.Created(c, "refund requested", toRefundResponse(ref))
}

func (h *handler) listRefunds(c *fiber.Ctx) error {
	pag := response.ParsePaginationRequest(c)
	refunds, total, err := h.svc.ListRefunds(c.Context(), pag.Page, pag.PageSize)
	if err != nil {
		return err
	}
	res := make([]RefundResponse, 0, len(refunds))
	for i := range refunds {
		res = append(res, toRefundResponse(&refunds[i]))
	}
	pagination := response.GeneratePagination(
		response.GetBaseURL(c),
		"/api/v1/payments/refunds",
		pag.Page, pag.PageSize, total,
		nil,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) approveRefund(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid refund id", nil)
	}
	approvedBy := middleware.GetUserID(c)
	ref, err := h.svc.ApproveRefund(c.Context(), id, approvedBy)
	if err != nil {
		return err
	}
	return response.Success(c, "refund approved", toRefundResponse(ref))
}

func (h *handler) rejectRefund(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "invalid refund id", nil)
	}
	rejectedBy := middleware.GetUserID(c)
	ref, err := h.svc.RejectRefund(c.Context(), id, rejectedBy)
	if err != nil {
		return err
	}
	return response.Success(c, "refund rejected", toRefundResponse(ref))
}
