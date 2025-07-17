package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/models"
)

// MockWidgetRepository is a mock implementation of WidgetRepository
type MockWidgetRepository struct {
	widgets map[string]*models.Widget
}

func NewMockWidgetRepository() *MockWidgetRepository {
	return &MockWidgetRepository{
		widgets: make(map[string]*models.Widget),
	}
}

func (m *MockWidgetRepository) Create(ctx context.Context, widget *models.Widget) error {
	m.widgets[widget.ID] = widget
	return nil
}

func (m *MockWidgetRepository) GetByID(ctx context.Context, id string) (*models.Widget, error) {
	widget, exists := m.widgets[id]
	if !exists {
		return nil, fmt.Errorf("widget not found")
	}
	return widget, nil
}

func (m *MockWidgetRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	var result []*models.Widget
	for _, widget := range m.widgets {
		if widget.OwnerID == userID {
			result = append(result, widget)
		}
	}
	return result, len(result), nil
}

func (m *MockWidgetRepository) Update(ctx context.Context, widget *models.Widget) error {
	m.widgets[widget.ID] = widget
	return nil
}

func (m *MockWidgetRepository) Delete(ctx context.Context, id string) error {
	delete(m.widgets, id)
	return nil
}

func (m *MockWidgetRepository) GetWidgetsByType(ctx context.Context, widgetType string, opts models.PaginationOptions) ([]*models.Widget, error) {
	var result []*models.Widget
	for _, widget := range m.widgets {
		if widget.Type == widgetType {
			result = append(result, widget)
		}
	}
	return result, nil
}

func (m *MockWidgetRepository) GetWidgetsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Widget, error) {
	var result []*models.Widget
	for _, widget := range m.widgets {
		if widget.Enabled == enabled {
			result = append(result, widget)
		}
	}
	return result, nil
}

// MockSubmissionRepository is a mock implementation of SubmissionRepository
type MockSubmissionRepository struct {
	submissions map[string][]*models.Submission
}

func NewMockSubmissionRepository() *MockSubmissionRepository {
	return &MockSubmissionRepository{
		submissions: make(map[string][]*models.Submission),
	}
}

func (m *MockSubmissionRepository) Create(ctx context.Context, submission *models.Submission) error {
	m.submissions[submission.WidgetID] = append(m.submissions[submission.WidgetID], submission)
	return nil
}

func (m *MockSubmissionRepository) GetByWidgetID(ctx context.Context, widgetID string, opts models.PaginationOptions) ([]*models.Submission, int, error) {
	submissions, exists := m.submissions[widgetID]
	if !exists {
		return []*models.Submission{}, 0, nil
	}
	return submissions, len(submissions), nil
}

func (m *MockSubmissionRepository) GetByID(ctx context.Context, widgetID, submissionID string) (*models.Submission, error) {
	submissions, exists := m.submissions[widgetID]
	if !exists {
		return nil, fmt.Errorf("submission not found")
	}
	for _, submission := range submissions {
		if submission.ID == submissionID {
			return submission, nil
		}
	}
	return nil, fmt.Errorf("submission not found")
}

func (m *MockSubmissionRepository) UpdateTTL(ctx context.Context, userID string, newTTL time.Duration) error {
	return nil
}

func (m *MockSubmissionRepository) UpdateWidgetSubmissionsTTL(ctx context.Context, widgetID string, ttlDays int) error {
	return nil
}

func TestExportService_ExportSubmissions(t *testing.T) {
	ctx := context.Background()
	widgetID := "test-widget-id"
	userID := "test-user-id"

	// Create mock repositories
	mockWidgetRepo := NewMockWidgetRepository()
	mockSubmissionRepo := NewMockSubmissionRepository()

	// Create export service
	exportService := NewExportService(mockSubmissionRepo, mockWidgetRepo)

	// Prepare test data
	widget := &models.Widget{
		ID:      widgetID,
		OwnerID: userID,
		Name:    "Test Widget",
		Type:    "contact",
	}

	submissions := []*models.Submission{
		{
			ID:       "sub1",
			WidgetID: widgetID,
			Data: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:       "sub2",
			WidgetID: widgetID,
			Data: map[string]interface{}{
				"name":    "Jane Smith",
				"email":   "jane@example.com",
				"message": "Hello world",
			},
			CreatedAt: time.Now().Add(-time.Hour),
		},
	}

	// Setup test data in mocks
	mockWidgetRepo.widgets[widgetID] = widget
	mockSubmissionRepo.submissions[widgetID] = submissions

	t.Run("Export as JSON", func(t *testing.T) {
		options := models.ExportOptions{
			Format: "json",
		}

		data, filename, err := exportService.ExportSubmissions(ctx, widgetID, userID, options)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected data to not be empty")
		}
		if !strings.Contains(filename, ".json") {
			t.Errorf("Expected filename to contain '.json', got: %s", filename)
		}
		dataStr := string(data)
		if !strings.Contains(dataStr, "John Doe") {
			t.Error("Expected data to contain 'John Doe'")
		}
		if !strings.Contains(dataStr, "Jane Smith") {
			t.Error("Expected data to contain 'Jane Smith'")
		}
	})

	t.Run("Export as CSV", func(t *testing.T) {
		options := models.ExportOptions{
			Format: "csv",
		}

		data, filename, err := exportService.ExportSubmissions(ctx, widgetID, userID, options)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected data to not be empty")
		}
		if !strings.Contains(filename, ".csv") {
			t.Errorf("Expected filename to contain '.csv', got: %s", filename)
		}
		csvData := string(data)
		if !strings.Contains(csvData, "John Doe") {
			t.Error("Expected data to contain 'John Doe'")
		}
		if !strings.Contains(csvData, "Jane Smith") {
			t.Error("Expected data to contain 'Jane Smith'")
		}
		if !strings.Contains(csvData, "name,email") {
			t.Error("Expected data to contain headers 'name,email'")
		}
	})

	t.Run("Export as XLSX", func(t *testing.T) {
		options := models.ExportOptions{
			Format: "xlsx",
		}

		data, filename, err := exportService.ExportSubmissions(ctx, widgetID, userID, options)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected data to not be empty")
		}
		if !strings.Contains(filename, ".xlsx") {
			t.Errorf("Expected filename to contain '.xlsx', got: %s", filename)
		}
		// XLSX data is binary, so we just check it's not empty
		if len(data) <= 0 {
			t.Error("Expected XLSX data length to be greater than 0")
		}
	})

	t.Run("Unauthorized access", func(t *testing.T) {
		wrongUserID := "wrong-user-id"
		options := models.ExportOptions{
			Format: "json",
		}

		_, _, err := exportService.ExportSubmissions(ctx, widgetID, wrongUserID, options)

		if err == nil {
			t.Error("Expected error for unauthorized access")
		}
		if !strings.Contains(err.Error(), "unauthorized") {
			t.Errorf("Expected error to contain 'unauthorized', got: %v", err)
		}
	})

	t.Run("Invalid format", func(t *testing.T) {
		options := models.ExportOptions{
			Format: "invalid",
		}

		_, _, err := exportService.ExportSubmissions(ctx, widgetID, userID, options)

		if err == nil {
			t.Error("Expected error for invalid format")
		}
		if !strings.Contains(err.Error(), "unsupported format") {
			t.Errorf("Expected error to contain 'unsupported format', got: %v", err)
		}
	})
}

func TestExportService_CollectFieldNames(t *testing.T) {
	exportService := &ExportService{}

	submissions := []*models.Submission{
		{
			Data: map[string]interface{}{
				"name":  "John",
				"email": "john@example.com",
			},
		},
		{
			Data: map[string]interface{}{
				"name":    "Jane",
				"email":   "jane@example.com",
				"message": "Hello",
			},
		},
		{
			Data: map[string]interface{}{
				"name":  "Bob",
				"phone": "123-456-7890",
			},
		},
	}

	fieldNames := exportService.collectFieldNames(submissions)

	// Check if all expected field names are present
	expectedFields := []string{"name", "email", "message", "phone"}
	for _, expected := range expectedFields {
		found := false
		for _, field := range fieldNames {
			if field == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected field '%s' not found in fieldNames", expected)
		}
	}

	// Check length
	if len(fieldNames) != 4 {
		t.Errorf("Expected 4 field names, got %d", len(fieldNames))
	}
}

func TestExportService_FormatValue(t *testing.T) {
	exportService := &ExportService{}

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil", nil, ""},
		{"slice", []string{"a", "b"}, `["a","b"]`},
		{"map", map[string]interface{}{"key": "value"}, `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exportService.formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
