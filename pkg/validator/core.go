package validator

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground/validator with extra features
// such as custom translators and options filtering.
type Validator struct {
	v          *validator.Validate
	translator MessageTranslator
	mu         sync.RWMutex
}

// New creates a fresh Validator instance with defaults.
func New() *Validator {
	v := validator.New()

	// Respect json tag names, fall back to snake_case
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
		return toSnakeCase(fld.Name)
	})

	return &Validator{
		v:          v,
		translator: defaultTranslator,
	}
}

// WithTranslator replaces the message translator.
func (v *Validator) WithTranslator(translator MessageTranslator) *Validator {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.translator = translator
	return v
}

// RegisterValidation proxies to the underlying validator.
func (v *Validator) RegisterValidation(tag string, fn validator.Func, callValidationEvenIfNull ...bool) error {
	return v.v.RegisterValidation(tag, fn, callValidationEvenIfNull...)
}

// RegisterStructValidation proxies struct-level validations.
func (v *Validator) RegisterStructValidation(fn validator.StructLevelFunc, types ...any) {
	v.v.RegisterStructValidation(fn, types...)
}

// Validate checks a struct according to tags and options.
func (v *Validator) Validate(data any, opts ...ValidationOptions) []FieldError {
	err := v.v.Struct(data)
	if err == nil {
		return nil
	}

	var validationOpts ValidationOptions
	if len(opts) > 0 {
		validationOpts = opts[0]
	}

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		return v.processValidationErrors(validationErrors, data, validationOpts)
	}

	return nil
}

// processValidationErrors converts validator errors into FieldError slices,
// applying options and invoking nested validations where appropriate.
func (v *Validator) processValidationErrors(validationErrors validator.ValidationErrors, data any, opts ValidationOptions) []FieldError {
	var errors []FieldError

	for _, fe := range validationErrors {
		fieldErr := FieldError{
			Field:   fe.Field(),
			Tag:     fe.Tag(),
			Message: v.translator(fe.Field(), fe.Tag(), fe.Param()),
		}

		if opts.IncludeValue {
			fieldErr.Value = fe.Value()
		}

		// Handle nested struct errors
		if fe.Type().Kind() == reflect.Struct {
			if nestedValue := v.getNestedValue(data, fe.Namespace()); nestedValue != nil {
				nestedErrors := v.Validate(nestedValue, opts)
				if len(nestedErrors) > 0 {
					fieldErr.Nested = nestedErrors
				}
			}
		}

		// Filter fields if specified
		if len(opts.Fields) > 0 && !contains(opts.Fields, fieldErr.Field) {
			continue
		}

		// Skip fields if specified
		if len(opts.SkipFields) > 0 && contains(opts.SkipFields, fieldErr.Field) {
			continue
		}

		errors = append(errors, fieldErr)
	}

	return errors
}

// ValidateVar validates a single variable against a tag string.
func (v *Validator) ValidateVar(field any, tag string, opts ...ValidationOptions) error {
	return v.v.Var(field, tag)
}

// getNestedValue is a placeholder for extracting a nested struct value based
// on the namespace path from validator.ValidationErrors. Implementation may
// use reflection to traverse the struct hierarchy.
func (v *Validator) getNestedValue(data any, namespace string) any {
	return nil
}
