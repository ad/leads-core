package validation

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/ad/leads-core/internal/models"
)

func TestSchemaValidator_Initialization(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	if validator == nil {
		t.Fatal("Expected validator to be non-nil")
	}

	// Check if schemas were loaded
	if len(validator.schemas) == 0 {
		t.Error("Expected schemas to be loaded, but found none")
	}
}

func TestSchemaValidator_ValidateRequest(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		schemaName  string
		requestBody string
		expectError bool
	}{
		{
			name:        "valid widget creation",
			schemaName:  "widget-create",
			requestBody: `{"type":"lead-form","name":"Test Widget","isVisible":true,"config":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}`,
			expectError: false,
		},
		{
			name:        "invalid widget creation - missing type",
			schemaName:  "widget-create",
			requestBody: `{"name":"Test Widget","isVisible":true,"config":{"name":"text"}}`,
			expectError: true,
		},
		{
			name:        "invalid widget creation - empty name",
			schemaName:  "widget-create",
			requestBody: `{"type":"lead-form","name":"","isVisible":true,"config":{"name":"text"}}`,
			expectError: true,
		},
		{
			name:        "valid submission",
			schemaName:  "submission",
			requestBody: `{"data":{"name":"John Doe","email":"john@example.com"}}`,
			expectError: false,
		},
		{
			name:        "invalid submission - missing data",
			schemaName:  "submission",
			requestBody: `{}`,
			expectError: true,
		},
		{
			name:        "valid event",
			schemaName:  "event",
			requestBody: `{"type":"view"}`,
			expectError: false,
		},
		{
			name:        "invalid event - wrong type",
			schemaName:  "event",
			requestBody: `{"type":"invalid"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("POST", "/test", bytes.NewBufferString(tt.requestBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Validate request
			data, err := validator.ValidateRequest(req, tt.schemaName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for invalid request, but got none")
				}
				if data != nil {
					t.Errorf("Expected nil data for invalid request, but got: %v", data)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for valid request, but got: %v", err)
				}
				if data == nil {
					t.Errorf("Expected data for valid request, but got nil")
				}
			}
		})
	}
}

func TestSchemaValidator_NonExistentSchema(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	req, err := http.NewRequest("POST", "/test", bytes.NewBufferString(`{"test": "data"}`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = validator.ValidateRequest(req, "non-existent-schema")
	if err == nil {
		t.Error("Expected error for non-existent schema, but got none")
	}
}

func TestSchemaValidator_InvalidJSON(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	req, err := http.NewRequest("POST", "/test", bytes.NewBufferString(`invalid json`))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, err = validator.ValidateRequest(req, "widget-create")
	if err == nil {
		t.Error("Expected error for invalid JSON, but got none")
	}
}

func TestValidationError_Error(t *testing.T) {
	validationErr := &ValidationError{
		Errors: []*models.FieldError{
			{Field: "name", Message: "Field 'name' is required"},
			{Field: "email", Message: "Field 'email' is invalid"},
		},
	}

	errMsg := validationErr.Error()
	expectedMsg := "validation failed: [{name Field 'name' is required} {email Field 'email' is invalid}]"

	if errMsg != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, errMsg)
	}
}

func TestSchemaValidator_ValidateAndDecode(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Test struct for decoding
	type TestWidget struct {
		Type      string                 `json:"type"`
		Name      string                 `json:"name"`
		IsVisible bool                   `json:"isVisible"`
		Config    map[string]interface{} `json:"config"`
	}

	requestBody := `{"type":"lead-form","name":"Test Widget","isVisible":true,"config":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}`
	req, err := http.NewRequest("POST", "/test", bytes.NewBufferString(requestBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var widget TestWidget
	err = validator.ValidateAndDecode(req, "widget-create", &widget)
	if err != nil {
		t.Fatalf("Failed to validate and decode: %v", err)
	}

	// Check decoded values
	if widget.Type != "lead-form" {
		t.Errorf("Expected type 'lead-form', got '%s'", widget.Type)
	}
	if widget.Name != "Test Widget" {
		t.Errorf("Expected name 'Test Widget', got '%s'", widget.Name)
	}
	if !widget.IsVisible {
		t.Errorf("Expected isVisible to be true, got %v", widget.IsVisible)
	}
}
