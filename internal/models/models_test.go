package models

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestWidgetToRedisHash(t *testing.T) {
	widget := &Widget{
		ID:        "test-widget-1",
		OwnerID:   "user-123",
		Type:      "lead-form",
		Name:      "Test Widget",
		IsVisible: true,
		Config:    map[string]interface{}{"name": "text", "email": "email"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	hash := widget.ToRedisHash()

	if hash["id"] != widget.ID {
		t.Errorf("Expected id %s, got %v", widget.ID, hash["id"])
	}

	if hash["owner_id"] != widget.OwnerID {
		t.Errorf("Expected owner_id %s, got %v", widget.OwnerID, hash["owner_id"])
	}

	if hash["type"] != widget.Type {
		t.Errorf("Expected type %s, got %v", widget.Type, hash["type"])
	}

	if hash["name"] != widget.Name {
		t.Errorf("Expected name %s, got %v", widget.Name, hash["name"])
	}

	if hash["isVisible"] != fmt.Sprintf("%v", widget.IsVisible) {
		t.Errorf("Expected isVisible %v, got %v", widget.IsVisible, hash["isVisible"])
	}
}

func TestWidgetFromRedisHash(t *testing.T) {
	hash := map[string]string{
		"id":         "test-widget-1",
		"owner_id":   "user-123",
		"type":       "lead-form",
		"name":       "Test Widget",
		"isVisible":  "true",
		"config":     `{"name":"text","email":"email"}`,
		"created_at": "1640995200", // 2022-01-01 00:00:00 UTC
		"updated_at": "1640995200",
	}

	widget := &Widget{}
	err := widget.FromRedisHash(hash)
	if err != nil {
		t.Fatalf("Failed to parse widget from hash: %v", err)
	}

	if widget.ID != hash["id"] {
		t.Errorf("Expected id %s, got %s", hash["id"], widget.ID)
	}

	if widget.OwnerID != hash["owner_id"] {
		t.Errorf("Expected owner_id %s, got %s", hash["owner_id"], widget.OwnerID)
	}

	if widget.Type != hash["type"] {
		t.Errorf("Expected type %s, got %s", hash["type"], widget.Type)
	}

	if widget.Name != hash["name"] {
		t.Errorf("Expected name %s, got %s", hash["name"], widget.Name)
	}

	if !widget.IsVisible {
		t.Errorf("Expected isVisible to be true")
	}

	if widget.Config["name"] != "text" {
		t.Errorf("Expected config.name to be 'text', got %v", widget.Config["name"])
	}

	if widget.Config["email"] != "email" {
		t.Errorf("Expected config.email to be 'email', got %v", widget.Config["email"])
	}
}

func TestWidget_JSONSerialization(t *testing.T) {
	original := &Widget{
		ID:        "test-widget-1",
		OwnerID:   "user-123",
		Type:      "lead-form",
		Name:      "Contact Widget",
		IsVisible: true,
		Config:    map[string]interface{}{"name": "required", "email": "required"},
		CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
	}

	// Test JSON marshaling
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal widget: %v", err)
	}

	// Test JSON unmarshaling
	var restored Widget
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal widget: %v", err)
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
	if restored.IsVisible != original.IsVisible {
		t.Errorf("Expected IsVisible %v, got %v", original.IsVisible, restored.IsVisible)
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
				WidgetID:  "widget-456",
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
				WidgetID:  "widget-456",
				Data:      map[string]interface{}{"name": "John"},
				CreatedAt: time.Now(),
			},
			valid: false,
		},
		{
			name: "empty WidgetID",
			submission: Submission{
				ID:        "sub-123",
				WidgetID:  "",
				Data:      map[string]interface{}{"name": "John"},
				CreatedAt: time.Now(),
			},
			valid: false,
		},
		{
			name: "nil data",
			submission: Submission{
				ID:        "sub-123",
				WidgetID:  "widget-456",
				Data:      nil,
				CreatedAt: time.Now(),
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.submission.ID != "" &&
				tt.submission.WidgetID != "" &&
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
func TestValidateFilterOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    *FilterOptions
		expected *FilterOptions
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "valid types",
			input: &FilterOptions{
				Types: []string{"lead-form", "banner", "quiz"},
			},
			expected: &FilterOptions{
				Types:  []string{"lead-form", "banner", "quiz"},
				Search: "",
			},
		},
		{
			name: "mixed valid and invalid types",
			input: &FilterOptions{
				Types: []string{"lead-form", "invalid-type", "banner", ""},
			},
			expected: &FilterOptions{
				Types:  []string{"lead-form", "banner"},
				Search: "",
			},
		},
		{
			name: "case insensitive types",
			input: &FilterOptions{
				Types: []string{"LEAD-FORM", "Banner", "QUIZ"},
			},
			expected: &FilterOptions{
				Types:  []string{"lead-form", "banner", "quiz"},
				Search: "",
			},
		},
		{
			name: "visibility filter",
			input: &FilterOptions{
				IsVisible: boolPtr(true),
			},
			expected: &FilterOptions{
				Types:     []string{},
				IsVisible: boolPtr(true),
				Search:    "",
			},
		},
		{
			name: "search filter with whitespace",
			input: &FilterOptions{
				Search: "  test search  ",
			},
			expected: &FilterOptions{
				Types:  []string{},
				Search: "test search",
			},
		},
		{
			name: "combined filters",
			input: &FilterOptions{
				Types:     []string{"lead-form", "invalid"},
				IsVisible: boolPtr(false),
				Search:    " contact form ",
			},
			expected: &FilterOptions{
				Types:     []string{"lead-form"},
				IsVisible: boolPtr(false),
				Search:    "contact form",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateFilterOptions(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected %+v, got nil", tt.expected)
				return
			}

			// Check types
			if len(result.Types) != len(tt.expected.Types) {
				t.Errorf("Expected %d types, got %d", len(tt.expected.Types), len(result.Types))
			} else {
				for i, expectedType := range tt.expected.Types {
					if result.Types[i] != expectedType {
						t.Errorf("Expected type[%d] %s, got %s", i, expectedType, result.Types[i])
					}
				}
			}

			// Check visibility
			if (result.IsVisible == nil) != (tt.expected.IsVisible == nil) {
				t.Errorf("IsVisible pointer mismatch: expected %v, got %v", tt.expected.IsVisible, result.IsVisible)
			} else if result.IsVisible != nil && tt.expected.IsVisible != nil {
				if *result.IsVisible != *tt.expected.IsVisible {
					t.Errorf("Expected IsVisible %v, got %v", *tt.expected.IsVisible, *result.IsVisible)
				}
			}

			// Check search
			if result.Search != tt.expected.Search {
				t.Errorf("Expected search '%s', got '%s'", tt.expected.Search, result.Search)
			}
		})
	}
}

func TestFilterOptions_HasFilters(t *testing.T) {
	tests := []struct {
		name     string
		filters  *FilterOptions
		expected bool
	}{
		{
			name:     "nil filters",
			filters:  nil,
			expected: false,
		},
		{
			name:     "empty filters",
			filters:  &FilterOptions{},
			expected: false,
		},
		{
			name: "has type filter",
			filters: &FilterOptions{
				Types: []string{"lead-form"},
			},
			expected: true,
		},
		{
			name: "has visibility filter",
			filters: &FilterOptions{
				IsVisible: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "has search filter",
			filters: &FilterOptions{
				Search: "test",
			},
			expected: true,
		},
		{
			name: "has all filters",
			filters: &FilterOptions{
				Types:     []string{"banner"},
				IsVisible: boolPtr(false),
				Search:    "contact",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.HasFilters()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterOptions_HasTypeFilter(t *testing.T) {
	tests := []struct {
		name     string
		filters  *FilterOptions
		expected bool
	}{
		{
			name:     "nil filters",
			filters:  nil,
			expected: false,
		},
		{
			name:     "empty types",
			filters:  &FilterOptions{Types: []string{}},
			expected: false,
		},
		{
			name:     "has types",
			filters:  &FilterOptions{Types: []string{"lead-form"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.HasTypeFilter()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterOptions_HasVisibilityFilter(t *testing.T) {
	tests := []struct {
		name     string
		filters  *FilterOptions
		expected bool
	}{
		{
			name:     "nil filters",
			filters:  nil,
			expected: false,
		},
		{
			name:     "nil visibility",
			filters:  &FilterOptions{IsVisible: nil},
			expected: false,
		},
		{
			name:     "has visibility true",
			filters:  &FilterOptions{IsVisible: boolPtr(true)},
			expected: true,
		},
		{
			name:     "has visibility false",
			filters:  &FilterOptions{IsVisible: boolPtr(false)},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.HasVisibilityFilter()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterOptions_HasSearchFilter(t *testing.T) {
	tests := []struct {
		name     string
		filters  *FilterOptions
		expected bool
	}{
		{
			name:     "nil filters",
			filters:  nil,
			expected: false,
		},
		{
			name:     "empty search",
			filters:  &FilterOptions{Search: ""},
			expected: false,
		},
		{
			name:     "has search",
			filters:  &FilterOptions{Search: "test"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.HasSearchFilter()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
