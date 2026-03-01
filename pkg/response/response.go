package response

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type Meta struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Response struct {
	Success bool `json:"success"`
	Meta    Meta `json:"meta"`
	Data    any  `json:"data,omitempty"`
}

func Success(c *fiber.Ctx, message string, data any) error {
	return c.Status(http.StatusOK).JSON(Response{
		Success: true,
		Meta:    Meta{Code: "OK", Message: message},
		Data:    data,
	})
}

func Created(c *fiber.Ctx, message string, data any) error {
	return c.Status(http.StatusCreated).JSON(Response{
		Success: true,
		Meta:    Meta{Code: "CREATED", Message: message},
		Data:    data,
	})
}

func BadRequest(c *fiber.Ctx, message string, details any) error {
	return c.Status(http.StatusBadRequest).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "INVALID_REQUEST", Message: message, Details: details},
	})
}

func Unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusUnauthorized).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "UNAUTHORIZED", Message: message},
	})
}

func Forbidden(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusForbidden).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "FORBIDDEN", Message: message},
	})
}

func NotFound(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusNotFound).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "NOT_FOUND", Message: message},
	})
}

func Conflict(c *fiber.Ctx, message string, details any) error {
	return c.Status(http.StatusConflict).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "CONFLICT", Message: message, Details: details},
	})
}

func InternalError(c *fiber.Ctx, message string) error {
	return c.Status(http.StatusInternalServerError).JSON(Response{
		Success: false,
		Meta:    Meta{Code: "INTERNAL_ERROR", Message: message},
	})
}
