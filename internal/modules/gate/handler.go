package gate

import (
	"bufio"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

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

// ── Token-first flow ──────────────────────────────────────────────────────────

// authenticate adalah endpoint setup first-time via gate_token.
// Screen kirim gate_token → dapat JWT long-lived → simpan di localStorage.
func (h *handler) authenticate(c *fiber.Ctx) error {
	var req GateAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	result, err := h.svc.Authenticate(c.Context(), req.GateToken)
	if err != nil {
		return err
	}

	return response.Success(c, "gate authenticated", result)
}

// ── QR Pairing flow ───────────────────────────────────────────────────────────

// requestPairing dibuat oleh screen gate saat pertama kali setup.
// Tidak butuh auth — screen belum punya token apapun.
func (h *handler) requestPairing(c *fiber.Ctx) error {
	ip := c.IP()

	result, err := h.svc.RequestPairing(c.Context(), ip)
	if err != nil {
		return err
	}

	return response.Created(c, "pairing code created", result)
}

// getPairingInfo dipanggil admin setelah scan QR.
// Butuh auth token admin.
func (h *handler) getPairingInfo(c *fiber.Ctx) error {
	code := c.Params("code")
	if len(code) != 6 {
		return response.BadRequest(c, "invalid pairing code", nil)
	}

	result, err := h.svc.GetPairingInfo(c.Context(), code)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", result)
}

// confirmPairing dipanggil admin untuk assign screen ke gate tertentu.
// Butuh auth token admin.
func (h *handler) confirmPairing(c *fiber.Ctx) error {
	code := c.Params("code")
	if len(code) != 6 {
		return response.BadRequest(c, "invalid pairing code", nil)
	}

	var req ConfirmPairingRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "validation failed", errs)
	}

	adminID := middleware.GetUserID(c)
	if adminID == uuid.Nil {
		return response.Unauthorized(c, "unauthorized")
	}

	result, err := h.svc.ConfirmPairing(c.Context(), code, req.GateID, adminID)
	if err != nil {
		return err
	}

	return response.Success(c, "pairing confirmed", result)
}

// listenPairing adalah SSE endpoint untuk screen.
// Screen subscribe ke sini dan nunggu JWT dikirim saat admin confirm.
// Tidak butuh auth — screen belum punya token apapun.
func (h *handler) listenPairing(c *fiber.Ctx) error {
	code := c.Params("code")
	if len(code) != 6 {
		return response.BadRequest(c, "invalid pairing code", nil)
	}

	ch, err := h.svc.ListenPairing(c.Context(), code)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Kirim heartbeat setiap 15 detik supaya koneksi tidak putus
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case jwt, ok := <-ch:
				if !ok {
					// Channel di-close (koneksi lama di-kick karena ada listener baru)
					fmt.Fprintf(w, "event: kicked\ndata: {\"reason\":\"new_listener_connected\"}\n\n")
					w.Flush()
					return
				}
				// Admin sudah confirm — kirim JWT ke screen
				fmt.Fprintf(w, "event: confirmed\ndata: {\"token\":\"%s\"}\n\n", jwt)
				w.Flush()
				return

			case <-ticker.C:
				fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
				w.Flush()

			case <-c.Context().Done():
				return
			}
		}
	}))

	return nil
}
