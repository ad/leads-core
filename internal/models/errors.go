package models

// FieldError represents a single validation error
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
