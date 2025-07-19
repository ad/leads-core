package services

import (
	"context"
	"testing"

	"github.com/ad/leads-core/internal/models"
	"github.com/google/uuid"
)

func TestGenerateWidgetID(t *testing.T) {
	service := &WidgetService{}

	userID := "test-user-123"

	// Generate multiple IDs for the same user
	id1 := service.generateWidgetID(userID)
	id2 := service.generateWidgetID(userID)

	// Verify both are valid UUIDs
	_, err1 := uuid.Parse(id1)
	_, err2 := uuid.Parse(id2)

	if err1 != nil {
		t.Errorf("First generated ID is not a valid UUID: %v", err1)
	}

	if err2 != nil {
		t.Errorf("Second generated ID is not a valid UUID: %v", err2)
	}

	// Verify they are different (due to different timestamps)
	if id1 == id2 {
		t.Error("Generated IDs should be different due to different timestamps")
	}

	// Verify consistent format
	if len(id1) != 36 || len(id2) != 36 {
		t.Errorf("UUID length should be 36 characters, got %d and %d", len(id1), len(id2))
	}
}

func TestGenerateSubmissionID(t *testing.T) {
	service := &WidgetService{}

	widgetID := "550e8400-e29b-41d4-a716-446655440000"

	// Generate multiple IDs for the same widget
	id1 := service.generateSubmissionID(widgetID)
	id2 := service.generateSubmissionID(widgetID)

	// Verify both are valid UUIDs
	_, err1 := uuid.Parse(id1)
	_, err2 := uuid.Parse(id2)

	if err1 != nil {
		t.Errorf("First generated submission ID is not a valid UUID: %v", err1)
	}

	if err2 != nil {
		t.Errorf("Second generated submission ID is not a valid UUID: %v", err2)
	}

	// Verify they are different (due to different timestamps)
	if id1 == id2 {
		t.Error("Generated submission IDs should be different due to different timestamps")
	}
}

func TestUUIDUniquenessAcrossUsers(t *testing.T) {
	service := &WidgetService{}

	user1ID := "user-1"
	user2ID := "user-2"

	// Generate IDs for different users
	widget1 := service.generateWidgetID(user1ID)
	widget2 := service.generateWidgetID(user2ID)

	// They should be different
	if widget1 == widget2 {
		t.Error("Widget IDs for different users should be different")
	}

	// Both should be valid UUIDs
	_, err1 := uuid.Parse(widget1)
	_, err2 := uuid.Parse(widget2)

	if err1 != nil || err2 != nil {
		t.Error("Generated IDs should be valid UUIDs")
	}
}

// Use the existing MockWidgetRepository from export_service_test.go

func TestGetUserWidgets_WithoutFilters(t *testing.T) {
	mockRepo := NewMockWidgetRepository()
	service := &WidgetService{
		widgetRepo: mockRepo,
	}

	ctx := context.Background()
	userID := "test-user"
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
	}

	_, _, err := service.GetUserWidgets(ctx, userID, opts)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockRepo.getByUserIDWithFiltersCalled {
		t.Error("Expected GetByUserIDWithFilters to be called")
	}

	if mockRepo.lastUserID != userID {
		t.Errorf("Expected userID %s, got %s", userID, mockRepo.lastUserID)
	}
}

func TestGetUserWidgets_WithValidFilters(t *testing.T) {
	mockRepo := NewMockWidgetRepository()
	service := &WidgetService{
		widgetRepo: mockRepo,
	}

	ctx := context.Background()
	userID := "test-user"
	isVisible := true
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
		Filters: &models.FilterOptions{
			Types:     []string{"lead-form", "banner"},
			IsVisible: &isVisible,
			Search:    "test widget",
		},
	}

	_, _, err := service.GetUserWidgets(ctx, userID, opts)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockRepo.getByUserIDWithFiltersCalled {
		t.Error("Expected GetByUserIDWithFilters to be called")
	}

	if mockRepo.lastFilters == nil {
		t.Error("Expected filters to be passed through")
	}

	if len(mockRepo.lastFilters.Types) != 2 {
		t.Errorf("Expected 2 types, got %d", len(mockRepo.lastFilters.Types))
	}

	if mockRepo.lastFilters.IsVisible == nil || *mockRepo.lastFilters.IsVisible != true {
		t.Error("Expected isVisible filter to be true")
	}

	if mockRepo.lastFilters.Search != "test widget" {
		t.Errorf("Expected search term 'test widget', got '%s'", mockRepo.lastFilters.Search)
	}
}

func TestGetUserWidgets_WithInvalidFilters(t *testing.T) {
	mockRepo := NewMockWidgetRepository()
	service := &WidgetService{
		widgetRepo: mockRepo,
	}

	ctx := context.Background()
	userID := "test-user"
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
		Filters: &models.FilterOptions{
			Types:  []string{"invalid-type", "lead-form", "another-invalid"},
			Search: "  test widget  ", // with extra spaces
		},
	}

	_, _, err := service.GetUserWidgets(ctx, userID, opts)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockRepo.getByUserIDWithFiltersCalled {
		t.Error("Expected GetByUserIDWithFilters to be called")
	}

	if mockRepo.lastFilters == nil {
		t.Error("Expected filters to be passed through")
	}

	// Should only have valid types after validation
	if len(mockRepo.lastFilters.Types) != 1 {
		t.Errorf("Expected 1 valid type after validation, got %d", len(mockRepo.lastFilters.Types))
	}

	if mockRepo.lastFilters.Types[0] != "lead-form" {
		t.Errorf("Expected 'lead-form', got '%s'", mockRepo.lastFilters.Types[0])
	}

	// Search should be trimmed
	if mockRepo.lastFilters.Search != "test widget" {
		t.Errorf("Expected trimmed search 'test widget', got '%s'", mockRepo.lastFilters.Search)
	}
}

func TestGetUserWidgets_WithEmptyFilters(t *testing.T) {
	mockRepo := NewMockWidgetRepository()
	service := &WidgetService{
		widgetRepo: mockRepo,
	}

	ctx := context.Background()
	userID := "test-user"
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
		Filters: &models.FilterOptions{
			Types:  []string{},
			Search: "",
		},
	}

	_, _, err := service.GetUserWidgets(ctx, userID, opts)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockRepo.getByUserIDWithFiltersCalled {
		t.Error("Expected GetByUserIDWithFilters to be called")
	}

	// Empty filters should still be passed through (repository will handle optimization)
	if mockRepo.lastFilters == nil {
		t.Error("Expected filters to be passed through even if empty")
	}
}

func TestGetUserWidgets_BackwardCompatibility(t *testing.T) {
	mockRepo := NewMockWidgetRepository()
	service := &WidgetService{
		widgetRepo: mockRepo,
	}

	ctx := context.Background()
	userID := "test-user"
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
		// No Filters field set (nil)
	}

	_, _, err := service.GetUserWidgets(ctx, userID, opts)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockRepo.getByUserIDWithFiltersCalled {
		t.Error("Expected GetByUserIDWithFilters to be called for backward compatibility")
	}

	if mockRepo.lastFilters != nil {
		t.Error("Expected filters to remain nil for backward compatibility")
	}
}
