package services

import (
	"testing"

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
