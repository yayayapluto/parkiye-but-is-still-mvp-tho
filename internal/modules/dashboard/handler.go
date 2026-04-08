package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/response"
)

type handler struct {
	svc ServicePort
}

func newHandler(svc ServicePort) *handler {
	return &handler{svc: svc}
}

// getStats handles GET /dashboard/stats
func (h *handler) getStats(c *fiber.Ctx) error {
	stats, err := h.svc.GetStats(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", stats)
}

// getUserRoleSummary handles GET /dashboard/user-role-summary
func (h *handler) getUserRoleSummary(c *fiber.Ctx) error {
	summary, err := h.svc.GetUserRoleSummary(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", summary)
}

func (h *handler) getOperatorDashboard(c *fiber.Ctx) error {
	data, err := h.svc.GetOperatorDashboard(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", data)
}

func (h *handler) getOwnerDashboard(c *fiber.Ctx) error {
	data, err := h.svc.GetOwnerDashboard(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", data)
}

func (h *handler) getAdminDashboard(c *fiber.Ctx) error {
	data, err := h.svc.GetAdminDashboard(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", data)
}

func (h *handler) getEngineerDashboard(c *fiber.Ctx) error {
	data, err := h.svc.GetEngineerDashboard(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", data)
}

func (h *handler) getCashierDashboard(c *fiber.Ctx) error {
	data, err := h.svc.GetCashierDashboard(c.Context())
	if err != nil {
		return err
	}
	return response.Success(c, "ok", data)
}
