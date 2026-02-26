package response

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/errors"
)

func Success(c *fiber.Ctx, message string, data any) error {
	return c.Status(http.StatusOK).JSON(SuccessResponse{
		Success: true,
		Meta: Meta{
			Code:    "OK",
			Message: message,
		},
		Data: data,
	})
}

func Created(c *fiber.Ctx, message string, data any) error {
	return c.Status(http.StatusCreated).JSON(SuccessResponse{
		Success: true,
		Meta: Meta{
			Code:    "CREATED",
			Message: message,
		},
		Data: data,
	})
}

func Paginated(c *fiber.Ctx, message string, data any, pagination Pagination) error {
	return c.Status(http.StatusOK).JSON(PaginatedResponse{
		Success:    true,
		Status:     http.StatusOK,
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}

func BadRequest(c *fiber.Ctx, message string, details any) error {
	return c.Status(http.StatusBadRequest).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "INVALID_REQUEST",
			Message: message,
			Details: details,
		},
	})
}

func Unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusUnauthorized).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "UNAUTHORIZED",
			Message: message,
		},
	})
}

func Forbidden(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusForbidden).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "FORBIDDEN",
			Message: message,
		},
	})
}

func NotFound(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusNotFound).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "NOT_FOUND",
			Message: message,
		},
	})
}

func Conflict(c *fiber.Ctx, message string, details any) error {
	return c.Status(http.StatusConflict).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "CONFLICT",
			Message: message,
			Details: details,
		},
	})
}

func InternalError(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "INTERNAL_ERROR",
			Message: message,
		},
	})
}

func AppErrorHandler(c *fiber.Ctx, appErr *errors.AppError) error {
	return c.Status(appErr.Status).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    string(appErr.Code),
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	if appErr, ok := errors.AsAppError(err); ok {
		return AppErrorHandler(c, appErr)
	}

	return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{
		Success: false,
		Meta: Meta{
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred",
		},
	})
}
