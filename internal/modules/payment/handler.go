package payment

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/include"
	"parkieee/pkg/middleware"
	"parkieee/pkg/response"
	"parkieee/pkg/validator"
)

type handler struct {
	svc            ServicePort
	v              *validator.Validator
	assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort
}

func newHandler(svc ServicePort, v *validator.Validator, assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort) *handler {
	return &handler{svc: svc, v: v, assignmentRepo: assignmentRepo}
}

func (h *handler) payCash(c *fiber.Ctx) error {
	var req PayCashRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	handledBy := middleware.GetUserID(c)
	p, err := h.svc.PayCash(c.Context(), req, handledBy)
	if err != nil {
		return err
	}
	return response.Created(c, "cash payment processed", toPaymentResponse(p, nil))
}

func (h *handler) initiateQRIS(c *fiber.Ctx) error {
	var req InitiateQRISRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	p, err := h.svc.InitiateQRIS(c.Context(), req)
	if err != nil {
		return err
	}
	return response.Created(c, "QRIS payment initiated", toPaymentResponse(p, nil))
}

func (h *handler) webhookMidtrans(c *fiber.Ctx) error {
	rawBody := c.Body()
	var payload MidtransWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return response.Success(c, "ok", nil)
	}
	_ = h.svc.HandleMidtransWebhook(c.Context(), rawBody, payload)
	return response.Success(c, "ok", nil)
}

func (h *handler) getPayment(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID pembayaran tidak valid", nil)
	}
	p, err := h.svc.GetPayment(c.Context(), id)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichPayment(c.Context(), p, includes)
	return response.Success(c, "ok", toPaymentResponse(p, enr))
}

func (h *handler) pollPaymentStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID pembayaran tidak valid", nil)
	}
	p, err := h.svc.PollPaymentStatus(c.Context(), id)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enr := h.svc.EnrichPayment(c.Context(), p, includes)
	return response.Success(c, "ok", toPaymentResponse(p, enr))
}

func (h *handler) listByTransaction(c *fiber.Ctx) error {
	txID, err := uuid.Parse(c.Params("txID"))
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}
	payments, err := h.svc.ListByTransaction(c.Context(), txID)
	if err != nil {
		return err
	}
	includes := include.ParseInclude(c)
	enrMap := h.svc.EnrichPaymentList(c.Context(), payments, includes)

	res := make([]PaymentResponse, 0, len(payments))
	for i := range payments {
		var enr *PaymentEnrichment
		if enrMap != nil {
			e := enrMap[payments[i].ID]
			enr = &e
		}
		res = append(res, toPaymentResponse(&payments[i], enr))
	}
	return response.Success(c, "ok", res)
}

func (h *handler) requestRefund(c *fiber.Ctx) error {
	var req RequestRefundRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
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
		nil,
	)
	return response.Paginated(c, "ok", res, pagination)
}

func (h *handler) approveRefund(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "ID refund tidak valid", nil)
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
		return response.BadRequest(c, "ID refund tidak valid", nil)
	}
	rejectedBy := middleware.GetUserID(c)
	ref, err := h.svc.RejectRefund(c.Context(), id, rejectedBy)
	if err != nil {
		return err
	}
	return response.Success(c, "refund rejected", toRefundResponse(ref))
}

// notifyKioskToCashier — kiosk kirim notif ke kasir yang di-assign ke gate tersebut.
// GateID diambil dari gate JWT claims, dipakai untuk lookup cashierUserID dari assignment.
func (h *handler) notifyKioskToCashier(c *fiber.Ctx) error {
	var event CashierEvent
	if err := c.BodyParser(&event); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	txID, err := uuid.Parse(event.TransactionID)
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}

	if stampErr := h.svc.StampCashierRequested(c.Context(), txID); stampErr != nil {
		_ = stampErr
	}

	// Enrich event dengan gate info dari JWT claims
	gateClaims := middleware.GetGateClaims(c)
	if gateClaims != nil {
		event.GateID = gateClaims.GateID.String()
		event.GateName = gateClaims.GateName

		// Lookup kasir yang di-assign ke gate ini
		assignment, err := h.assignmentRepo.FindByGateID(c.Context(), gateClaims.GateID)
		if err == nil {
			_ = h.svc.NotifyCashier(assignment.UserID, event)
		}
		// Kalau tidak ada assignment (manless gate), notif tidak dikirim — by design
	}

	return response.Success(c, "notified", nil)
}

func (h *handler) qrisSimPage(c *fiber.Ctx) error {
	qrURL := c.Query("url")
	if qrURL == "" {
		return response.BadRequest(c, "URL wajib diisi", nil)
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>QRIS Simulator</title></head>
<body style="font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#f5f5f5">
  <div style="text-align:center">
    <p style="color:#666;font-size:13px">Mengarahkan ke Midtrans Simulator...</p>
    <form id="f" method="POST" action="https://simulator.sandbox.midtrans.com/v2/qris/index">
      <input type="hidden" name="qrCodeUrl" value="` + qrURL + `">
    </form>
    <script>document.getElementById('f').submit();</script>
  </div>
</body>
</html>`
	return c.SendString(html)
}

func (h *handler) simulatePay(c *fiber.Ctx) error {
	var body struct {
		QRISImageURL string `json:"qris_image_url"`
	}
	if err := c.BodyParser(&body); err != nil || body.QRISImageURL == "" {
		return response.BadRequest(c, "URL gambar QRIS wajib diisi", nil)
	}
	if err := h.svc.SimulatePay(c.Context(), body.QRISImageURL); err != nil {
		return err
	}
	return response.Success(c, "simulate pay triggered", nil)
}

// pendingCashierRequests — kasir poll untuk detect request tunai dari kiosk.
// Hanya return request dari gate yang di-assign ke kasir yang login.
func (h *handler) pendingCashierRequests(c *fiber.Ctx) error {
	since := c.Query("since")
	userID := middleware.GetUserID(c)
	h.svc.TouchCashierSeen(userID)

	txs, err := h.svc.GetPendingCashierRequests(c.Context(), since, userID)
	if err != nil {
		return err
	}
	return response.Success(c, "ok", txs)
}

// listenAsCashier — kasir subscribe SSE, hanya terima notif dari gate yang di-assign ke dia.
func (h *handler) listenAsCashier(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	h.svc.TouchCashierSeen(userID)

	ch, cleanup := h.svc.ListenCashier(userID)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		defer cleanup()
		fmt.Fprintf(w, ": connected\n\n")
		w.Flush()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				b, _ := json.Marshal(event)
				fmt.Fprintf(w, "event: payment_request\ndata: %s\n\n", b)
				w.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				w.Flush()
			case <-c.Context().Done():
				return
			}
		}
	}))
	return nil
}

func (h *handler) notifyCashierToKiosk(c *fiber.Ctx) error {
	var event KioskEvent
	if err := c.BodyParser(&event); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	txID, err := uuid.Parse(event.TransactionID)
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}
	_ = h.svc.NotifyKiosk(txID, event)
	return response.Success(c, "notified", nil)
}

func (h *handler) listenAsKiosk(c *fiber.Ctx) error {
	txID, err := uuid.Parse(c.Params("txID"))
	if err != nil {
		return response.BadRequest(c, "ID transaksi tidak valid", nil)
	}

	ch, cleanup := h.svc.ListenKiosk(txID)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		defer cleanup()
		fmt.Fprintf(w, ": connected\n\n")
		w.Flush()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				b, _ := json.Marshal(event)
				fmt.Fprintf(w, "event: cashier_done\ndata: %s\n\n", b)
				w.Flush()
				return
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				w.Flush()
			case <-c.Context().Done():
				return
			}
		}
	}))
	return nil
}

// cashierStatus — kiosk cek apakah kasir yang di-assign ke gate-nya sedang online.
// Dipakai sebelum tampilkan opsi tunai pada mode with_cashier.
func (h *handler) cashierStatus(c *fiber.Ctx) error {
	gateClaims := middleware.GetGateClaims(c)
	if gateClaims == nil {
		return response.Unauthorized(c, "Informasi gate tidak ditemukan")
	}
	status := h.svc.GetCashierStatus(gateClaims.GateID)
	return response.Success(c, "ok", status)
}
