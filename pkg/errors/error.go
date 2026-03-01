package errors

import (
	"fmt"
	"net/http"
	"runtime"
)

type ErrorCode string

const (
	// Client Errors (4xx)
	ErrInvalidRequest     ErrorCode = "INVALID_REQUEST"
	ErrValidation         ErrorCode = "VALIDATION_ERROR"
	ErrUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrAccountLocked      ErrorCode = "ACCOUNT_LOCKED"
	ErrForbidden          ErrorCode = "FORBIDDEN"
	ErrPermissionDenied   ErrorCode = "PERMISSION_DENIED"
	ErrNotFound           ErrorCode = "NOT_FOUND"
	ErrConflict           ErrorCode = "CONFLICT"
	ErrDuplicate          ErrorCode = "DUPLICATE_ENTRY"
	ErrResourceInUse      ErrorCode = "RESOURCE_IN_USE"
	ErrRateLimited        ErrorCode = "RATE_LIMITED"

	ErrInternal         ErrorCode = "INTERNAL_ERROR"
	ErrDatabaseError    ErrorCode = "DATABASE_ERROR"
	ErrDecryptionFailed ErrorCode = "DECRYPTION_FAILED"
	ErrExternalService  ErrorCode = "EXTERNAL_SERVICE_ERROR"
)

type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
	Status  int       `json:"-"`
	Cause   error     `json:"-"` // Underlying error
	Stack   []byte    `json:"-"` // Stack trace
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

func (e *AppError) WithStack() *AppError {
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)
	e.Stack = stack[:n]
	return e
}

func New(code ErrorCode, message string) *AppError {
	status := statusFromCode(code)
	err := &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}

	if status >= 500 {
		return err.WithStack()
	}
	return err
}

func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}

	appErr := New(code, message)
	appErr.Cause = err

	// If it's already an AppError, preserve its details
	if existing, ok := err.(*AppError); ok {
		if appErr.Details == nil {
			appErr.Details = existing.Details
		}
	}

	return appErr
}

func statusFromCode(code ErrorCode) int {
	switch code {
	// 400 Bad Request
	case ErrInvalidRequest, ErrValidation:
		return http.StatusBadRequest

	// 401 Unauthorized
	case ErrUnauthorized, ErrInvalidCredentials, ErrAccountLocked:
		return http.StatusUnauthorized

	// 403 Forbidden
	case ErrForbidden, ErrPermissionDenied:
		return http.StatusForbidden

	// 404 Not Found
	case ErrNotFound:
		return http.StatusNotFound

	// 409 Conflict
	case ErrConflict, ErrDuplicate, ErrResourceInUse:
		return http.StatusConflict

	// 429 Too Many Requests
	case ErrRateLimited:
		return http.StatusTooManyRequests

	// 500 Internal Server Error
	case ErrInternal, ErrDatabaseError, ErrDecryptionFailed:
		return http.StatusInternalServerError

	// 502 Bad Gateway / 503 Service Unavailable
	case ErrExternalService:
		return http.StatusBadGateway

	default:
		return http.StatusInternalServerError
	}
}

// Helper to check if error is a specific code
func IsCode(err error, code ErrorCode) bool {
	var appErr *AppError
	if AsAppError(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// Generic AsAppError with target
func AsAppError(err error, target **AppError) bool {
	if err == nil {
		return false
	}
	*target, _ = err.(*AppError)
	return *target != nil
}
