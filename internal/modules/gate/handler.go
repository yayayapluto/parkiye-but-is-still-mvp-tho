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

func (h *handler) authenticate(c *fiber.Ctx) error {
	var req GateAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	result, err := h.svc.Authenticate(c.Context(), req.GateToken)
	if err != nil {
		return err
	}

	return response.Success(c, "gate authenticated", result)
}

func (h *handler) requestPairing(c *fiber.Ctx) error {
	result, err := h.svc.RequestPairing(c.Context(), c.IP())
	if err != nil {
		return err
	}
	return response.Created(c, "pairing code created", result)
}

func (h *handler) getPairingInfo(c *fiber.Ctx) error {
	code := c.Params("code")
	if len(code) != 6 {
		return response.BadRequest(c, "Kode pairing tidak valid", nil)
	}

	result, err := h.svc.GetPairingInfo(c.Context(), code)
	if err != nil {
		return err
	}

	return response.Success(c, "ok", result)
}

func (h *handler) confirmPairing(c *fiber.Ctx) error {
	code := c.Params("code")
	if len(code) != 6 {
		return response.BadRequest(c, "Kode pairing tidak valid", nil)
	}

	var req ConfirmPairingRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid", nil)
	}
	if errs := h.v.Validate(req); errs != nil {
		return response.BadRequest(c, "Validasi gagal", errs)
	}

	adminID := middleware.GetUserID(c)
	if adminID == uuid.Nil {
		return response.Unauthorized(c, "Tidak memiliki otorisasi")
	}

	result, err := h.svc.ConfirmPairing(c.Context(), code, req, adminID)
	if err != nil {
		return err
	}

	return response.Success(c, "pairing confirmed", result)
}

func (h *handler) listenPairing(c *fiber.Ctx) error {
	code := c.Params("code")
	if len(code) != 6 {
		return response.BadRequest(c, "Kode pairing tidak valid", nil)
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
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case payload, ok := <-ch:
				if !ok {
					fmt.Fprintf(w, "event: kicked\ndata: {\"reason\":\"new_listener_connected\"}\n\n")
					w.Flush()
					return
				}
				fmt.Fprintf(w, "event: confirmed\ndata: %s\n\n", payload)
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
