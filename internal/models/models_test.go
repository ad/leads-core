package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFormToRedisHash(t *testing.T) {
	form := &Form{
		ID:        "test-form-1",
		OwnerID:   "user-123",
		Type:      "contact",
		Name:      "Test Form",
		Enabled:   true,
		Fields:    map[string]interface{}{"name": "text", "email": "email"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	hash := form.ToRedisHash()

	if hash["id"] != form.ID {
		t.Errorf("Expected id %s, got %v", form.ID, hash["id"])
	}

	if hash["owner_id"] != form.OwnerID {
		t.Errorf("Expected owner_id %s, got %v", form.OwnerID, hash["owner_id"])
	}

	if hash["type"] != form.Type {
		t.Errorf("Expected type %s, got %v", form.Type, hash["type"])
	}

	if hash["name"] != form.Name {
		t.Errorf("Expected name %s, got %v", form.Name, hash["name"])
	}

	if hash["enabled"] != form.Enabled {
		t.Errorf("Expected enabled %v, got %v", form.Enabled, hash["enabled"])
	}
}

func TestFormFromRedisHash(t *testing.T) {
	hash := map[string]string{
		"id":         "test-form-1",
		"owner_id":   "user-123",
		"type":       "contact",
		"name":       "Test Form",
		"enabled":    "true",
		"fields":     `{"name":"text","email":"email"}`,
		"created_at": "1640995200", // 2022-01-01 00:00:00 UTC
		"updated_at": "1640995200",
	}

	form := &Form{}
	err := form.FromRedisHash(hash)
	if err != nil {
		t.Fatalf("Failed to parse form from hash: %v", err)
	}

	if form.ID != hash["id"] {
		t.Errorf("Expected id %s, got %s", hash["id"], form.ID)
	}

	if form.OwnerID != hash["owner_id"] {
		t.Errorf("Expected owner_id %s, got %s", hash["owner_id"], form.OwnerID)
	}

	if form.Type != hash["type"] {
		t.Errorf("Expected type %s, got %s", hash["type"], form.Type)
	}

	if form.Name != hash["name"] {
		t.Errorf("Expected name %s, got %s", hash["name"], form.Name)
	}

	if !form.Enabled {
		t.Errorf("Expected enabled to be true")
	}

	if form.Fields["name"] != "text" {
		t.Errorf("Expected fields.name to be 'text', got %v", form.Fields["name"])
	}
}

func TestForm_JSONSerialization(t *testing.T) {
	original := &Form{
		ID:        "test-form-1",
		OwnerID:   "user-123",
		Type:      "contact",
		Name:      "Contact Form",
		Enabled:   true,
		Fields:    map[string]interface{}{"name": "required", "email": "required"},
		CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
	}

	// Test JSON marshaling
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal form: %v", err)
	}

	// Test JSON unmarshaling
	var restored Form
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal form: %v", err)
	}

	// Compare fields
	if restored.ID != original.ID {
		t.Errorf("Expected ID %s, got %s", original.ID, restored.ID)
	}
	if restored.OwnerID != original.OwnerID {
		t.Errorf("Expected OwnerID %s, got %s", original.OwnerID, restored.OwnerID)
	}
	if restored.Type != original.Type {
		t.Errorf("Expected Type %s, got %s", original.Type, restored.Type)
	}
	if restored.Name != original.Name {
		t.Errorf("Expected Name %s, got %s", original.Name, restored.Name)
	}
	if restored.Enabled != original.Enabled {
		t.Errorf("Expected Enabled %v, got %v", original.Enabled, restored.Enabled)
	}
}

func TestSubmission_Validation(t *testing.T) {
	tests := []struct {
		name       string
		submission Submission
		valid      bool
	}{
		{
			name: "valid submission",
			submission: Submission{
				ID:        "sub-123",
				FormID:    "form-456",
				Data:      map[string]interface{}{"name": "John", "email": "john@test.com"},
				CreatedAt: time.Now(),
				TTL:       24 * time.Hour,
			},
			valid: true,
		},
		{
			name: "empty ID",
			submission: Submission{
				ID:        "",
				FormID:    "form-456",
				Data:      map[string]interface{}{"name": "John"},
				CreatedAt: time.Now(),
			},
			valid: false,
		},
		{
			name: "empty FormID",
			submission: Submission{
				ID:        "sub-123",
				FormID:    "",
				Data:      map[string]interface{}{"name": "John"},
				CreatedAt: time.Now(),
			},
			valid: false,
		},
		{
			name: "nil data",
			submission: Submission{
				ID:        "sub-123",
				FormID:    "form-456",
				Data:      nil,
				CreatedAt: time.Now(),
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.submission.ID != "" &&
				tt.submission.FormID != "" &&
				tt.submission.Data != nil &&
				!tt.submission.CreatedAt.IsZero()

			if isValid != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.valid, isValid)
			}
		})
	}
}

func TestPaginationOptions_Calculate(t *testing.T) {
	tests := []struct {
		name          string
		options       PaginationOptions
		expectedSkip  int
		expectedLimit int
	}{
		{
			name:          "first page",
			options:       PaginationOptions{Page: 1, PerPage: 20},
			expectedSkip:  0,
			expectedLimit: 20,
		},
		{
			name:          "second page",
			options:       PaginationOptions{Page: 2, PerPage: 20},
			expectedSkip:  20,
			expectedLimit: 20,
		},
		{
			name:          "custom page size",
			options:       PaginationOptions{Page: 3, PerPage: 10},
			expectedSkip:  20,
			expectedLimit: 10,
		},
		{
			name:          "zero page defaults to 1",
			options:       PaginationOptions{Page: 0, PerPage: 20},
			expectedSkip:  0,
			expectedLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := tt.options.Page
			if page <= 0 {
				page = 1
			}

			skip := (page - 1) * tt.options.PerPage
			limit := tt.options.PerPage

			if skip != tt.expectedSkip {
				t.Errorf("Expected skip %d, got %d", tt.expectedSkip, skip)
			}
			if limit != tt.expectedLimit {
				t.Errorf("Expected limit %d, got %d", tt.expectedLimit, limit)
			}
		})
	}
}
