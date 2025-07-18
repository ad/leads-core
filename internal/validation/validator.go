package validation

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ad/leads-core/internal/models"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed schemas/*.json
var schemaFS embed.FS

// SchemaValidator handles JSON schema validation
type SchemaValidator struct {
	schemas map[string]*gojsonschema.Schema
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() (*SchemaValidator, error) {
	validator := &SchemaValidator{
		schemas: make(map[string]*gojsonschema.Schema),
	}

	// Load all schemas
	schemaNames := []string{
		"widget-create.json",
		"widget-update.json",
		"widget-config-update.json",
		"submission.json",
		"event.json",
	}

	for _, schemaName := range schemaNames {
		schemaData, err := schemaFS.ReadFile("schemas/" + schemaName)
		if err != nil {
			return nil, fmt.Errorf("failed to read schema %s: %w", schemaName, err)
		}

		schemaLoader := gojsonschema.NewBytesLoader(schemaData)
		schema, err := gojsonschema.NewSchema(schemaLoader)
		if err != nil {
			return nil, fmt.Errorf("failed to compile schema %s: %w", schemaName, err)
		}

		// Remove .json extension for key
		key := schemaName[:len(schemaName)-5]
		validator.schemas[key] = schema
	}

	return validator, nil
}

// ValidateRequest validates request body against a schema
func (v *SchemaValidator) ValidateRequest(r *http.Request, schemaName string) (map[string]interface{}, error) {
	schema, exists := v.schemas[schemaName]
	if !exists {
		return nil, fmt.Errorf("schema %s not found", schemaName)
	}

	// Read and parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate against schema
	documentLoader := gojsonschema.NewGoLoader(data)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		// Collect validation errors
		var errors []*models.FieldError
		for _, desc := range result.Errors() {
			errors = append(errors, &models.FieldError{
				Field:   desc.Field(),
				Message: desc.Description(),
			})
		}
		return nil, &ValidationError{Errors: errors}
	}

	return data, nil
}

// ValidationError represents validation errors
type ValidationError struct {
	Errors []*models.FieldError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Errors)
}

// ValidateAndDecode validates request and decodes into target struct
func (v *SchemaValidator) ValidateAndDecode(r *http.Request, schemaName string, target interface{}) error {
	schema, exists := v.schemas[schemaName]
	if !exists {
		return fmt.Errorf("schema %s not found", schemaName)
	}

	// Read and parse request body once
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Parse JSON into target struct
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate the parsed data
	documentLoader := gojsonschema.NewGoLoader(target)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		// Collect validation errors
		var errors []*models.FieldError
		for _, desc := range result.Errors() {
			errors = append(errors, &models.FieldError{
				Field:   desc.Field(),
				Message: desc.Description(),
			})
		}
		return &ValidationError{Errors: errors}
	}

	return nil
}
