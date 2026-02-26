package validator

import (
	"fmt"
)

// MessageTranslator converts field/tag/param into a human-readable string.
type MessageTranslator func(field, tag, param string) string

// defaultTranslator provides minimal english messages.
func defaultTranslator(field, tag, param string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s does not meet minimum requirement", field)
	case "max":
		return fmt.Sprintf("%s exceeds maximum allowed", field)
	default:
		return fmt.Sprintf("%s failed validation", field)
	}
}

// contextAwareTranslator supplies more detail for numeric bounds and lists.
func contextAwareTranslator(field, tag, param string) string {
	switch tag {
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, param)
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, param)
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, param)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
