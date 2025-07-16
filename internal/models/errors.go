package models

// FieldError represents a single validation error
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// String returns string representation of FieldError
func (e *FieldError) String() string {
	return "{" + e.Field + " " + e.Message + "}"
}
