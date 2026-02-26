package validator

// FieldError represents a single validation failure.
type FieldError struct {
	Field   string      `json:"field"`              // The actual field name (from JSON tag or struct)
	Tag     string      `json:"tag"`                // Validation tag that failed
	Value   any         `json:"value,omitempty"`   // The actual value that failed
	Message string      `json:"message"`            // Human-readable message
	Nested  []FieldError `json:"nested,omitempty"`  // For nested structs
}

// ValidationOptions allow callers to tweak validator behavior.
type ValidationOptions struct {
	UseJSONTags bool                 // Use JSON tags for field names
	IncludeValue bool                // Include the invalid value in response
	FieldNameMapper func(string) string // Custom field name mapper
	Fields []string                  // Validate only these fields
	SkipFields []string              // Skip validation for these fields
}
