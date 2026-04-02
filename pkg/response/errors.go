package response

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	cErrors "parkieee/pkg/errors"
)

// ErrorHandler dipakai di fiber.Config{ErrorHandler: response.ErrorHandler}
func ErrorHandler(c *fiber.Ctx, err error) error {
	// AppError
	var appErr *cErrors.AppError
	if errors.As(err, &appErr) {
		msg := appErr.Message
		// Sembunyikan detail error internal dari user
		if appErr.Status >= 500 {
			msg = "Terjadi kesalahan pada server"
		}
		return c.Status(appErr.Status).JSON(Response{
			Success: false,
			Meta: Meta{
				Code:    string(appErr.Code),
				Message: msg,
				Details: appErr.Details,
			},
		})
	}

	// Fiber error (404, method not allowed, dll)
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(Response{
			Success: false,
			Meta:    Meta{Code: http.StatusText(fiberErr.Code), Message: fiberErr.Message},
		})
	}

	// Fallback
	return c.Status(http.StatusInternalServerError).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "INTERNAL_ERROR", Message: "Terjadi kesalahan yang tidak terduga"},
	})
}
