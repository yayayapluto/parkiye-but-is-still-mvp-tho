package validator

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	globalValidator *Validator
	once            sync.Once
)

// Global returns a shared Validator instance.
func Global() *Validator {
	once.Do(func() {
		globalValidator = New()
	})
	return globalValidator
}

// Validate convenience wrapper using the global validator.
func Validate(data any) []FieldError {
	return Global().Validate(data)
}

// ValidateWithOpts allows passing options along with a global validator call.
func ValidateWithOpts(data any, opts ValidationOptions) []FieldError {
	return Global().Validate(data, opts)
}

// RegisterValidation registers a custom tag on the global validator.
func RegisterValidation(tag string, fn validator.Func) error {
	return Global().RegisterValidation(tag, fn)
}
