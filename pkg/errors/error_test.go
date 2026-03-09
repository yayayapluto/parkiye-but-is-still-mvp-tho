package errors_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		code           errors.ErrorCode
		message        string
		expectedStatus int
		expectStack    bool
	}{
		{"validation error is 400", errors.ErrValidation, "invalid input", http.StatusBadRequest, false},
		{"not found is 404", errors.ErrNotFound, "not found", http.StatusNotFound, false},
		{"unauthorized is 401", errors.ErrUnauthorized, "unauthorized", http.StatusUnauthorized, false},
		{"forbidden is 403", errors.ErrForbidden, "forbidden", http.StatusForbidden, false},
		{"conflict is 409", errors.ErrConflict, "conflict", http.StatusConflict, false},
		{"internal error is 500 with stack", errors.ErrInternal, "internal", http.StatusInternalServerError, true},
		{"database error is 500 with stack", errors.ErrDatabaseError, "db fail", http.StatusInternalServerError, true},
		{"external service is 502 with stack", errors.ErrExternalService, "gateway fail", http.StatusBadGateway, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.code, tc.message)

			require.NotNil(t, err)
			assert.Equal(t, tc.code, err.Code)
			assert.Equal(t, tc.message, err.Message)
			assert.Equal(t, tc.expectedStatus, err.Status)
			assert.Contains(t, err.Error(), string(tc.code))
			assert.Contains(t, err.Error(), tc.message)

			if tc.expectStack {
				assert.NotEmpty(t, err.Stack)
			} else {
				assert.Empty(t, err.Stack)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	t.Run("wraps cause correctly", func(t *testing.T) {
		cause := fmt.Errorf("original error")
		wrapped := errors.Wrap(cause, errors.ErrDatabaseError, "wrapper message")

		require.NotNil(t, wrapped)
		assert.Equal(t, errors.ErrDatabaseError, wrapped.Code)
		assert.Equal(t, "wrapper message", wrapped.Message)
		assert.Equal(t, cause, wrapped.Unwrap())
		assert.Contains(t, wrapped.Error(), "original error")
	})

	t.Run("nil input returns nil", func(t *testing.T) {
		result := errors.Wrap(nil, errors.ErrDatabaseError, "should be nil")
		assert.Nil(t, result)
	})

	t.Run("propagates details from inner AppError", func(t *testing.T) {
		inner := errors.New(errors.ErrNotFound, "inner").WithDetails(map[string]string{"key": "val"})
		outer := errors.Wrap(inner, errors.ErrDatabaseError, "outer")

		require.NotNil(t, outer)
		assert.NotNil(t, outer.Details)
	})
}

func TestIsCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     errors.ErrorCode
		expected bool
	}{
		{"matches correct code", errors.New(errors.ErrNotFound, "not found"), errors.ErrNotFound, true},
		{"does not match wrong code", errors.New(errors.ErrNotFound, "not found"), errors.ErrInternal, false},
		{"nil error returns false", nil, errors.ErrNotFound, false},
		{"non-AppError returns false", fmt.Errorf("plain error"), errors.ErrNotFound, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, errors.IsCode(tc.err, tc.code))
		})
	}
}

func TestFromDB(t *testing.T) {
	tests := []struct {
		name         string
		input        error
		expectedCode errors.ErrorCode
		expectedNil  bool
	}{
		{"nil returns nil", nil, "", true},
		{"ErrRecordNotFound returns ErrNotFound", gorm.ErrRecordNotFound, errors.ErrNotFound, false},
		{"duplicate key returns ErrDuplicate", fmt.Errorf("duplicate key value violates"), errors.ErrDuplicate, false},
		{"generic db error returns ErrDatabaseError", fmt.Errorf("connection refused"), errors.ErrDatabaseError, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := errors.FromDB(tc.input, "record not found")

			if tc.expectedNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.True(t, errors.IsCode(result, tc.expectedCode))
		})
	}
}

func TestAppError_WithDetails(t *testing.T) {
	details := map[string]string{"field": "email", "reason": "already taken"}
	err := errors.New(errors.ErrConflict, "conflict").WithDetails(details)

	assert.Equal(t, details, err.Details)
}
