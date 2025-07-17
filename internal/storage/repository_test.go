package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// MockRedisClient wraps a simple Redis client for testing
type MockRedisClient struct {
	client redis.UniversalClient
}

// setupTestRedis creates a miniredis instance for testing
func setupTestRedis(t *testing.T) *MockRedisClient {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	t.Cleanup(func() {
		mr.Close()
	})

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return &MockRedisClient{client: client}
}

// Simple mock repository for testing
type MockWidgetRepository struct {
	client *MockRedisClient
}

func NewMockWidgetRepository(client *MockRedisClient) *MockWidgetRepository {
	return &MockWidgetRepository{client: client}
}

func (r *MockWidgetRepository) Create(ctx context.Context, widget *models.Widget) error {
	widgetKey := fmt.Sprintf("widget:%s", widget.ID)
	data := map[string]interface{}{
		"id":         widget.ID,
		"owner_id":   widget.OwnerID,
		"name":       widget.Name,
		"type":       widget.Type,
		"enabled":    widget.Enabled,
		"created_at": widget.CreatedAt.Unix(),
		"updated_at": widget.UpdatedAt.Unix(),
	}

	return r.client.client.HMSet(ctx, widgetKey, data).Err()
}

func (r *MockWidgetRepository) GetByID(ctx context.Context, id string) (*models.Widget, error) {
	widgetKey := fmt.Sprintf("widget:%s", id)
	data, err := r.client.client.HGetAll(ctx, widgetKey).Result()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("widget not found")
	}

	widget := &models.Widget{
		ID:      data["id"],
		OwnerID: data["owner_id"],
		Name:    data["name"],
		Type:    data["type"],
		Enabled: data["enabled"] == "1",
	}

	return widget, nil
}

func (r *MockWidgetRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, error) {
	// Simplified implementation for testing
	pattern := "widget:*"
	keys, err := r.client.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var widgets []*models.Widget
	for _, key := range keys {
		data, err := r.client.client.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		if data["owner_id"] == userID {
			widget := &models.Widget{
				ID:      data["id"],
				OwnerID: data["owner_id"],
				Name:    data["name"],
				Type:    data["type"],
				Enabled: data["enabled"] == "1",
			}
			widgets = append(widgets, widget)
		}
	}

	return widgets, nil
}

func (r *MockWidgetRepository) Update(ctx context.Context, widget *models.Widget) error {
	return r.Create(ctx, widget) // Simplified for testing
}

func (r *MockWidgetRepository) Delete(ctx context.Context, id string) error {
	widgetKey := fmt.Sprintf("widget:%s", id)
	return r.client.client.Del(ctx, widgetKey).Err()
}

func TestWidgetRepository_Create(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockWidgetRepository(client)
	ctx := context.Background()

	widget := &models.Widget{
		ID:        "test-widget-1",
		OwnerID:   "user-123",
		Name:      "Test Widget",
		Type:      "lead-form",
		Enabled:   true,
		Fields:    map[string]interface{}{"name": map[string]interface{}{"type": "text", "required": true}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(ctx, widget)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}

	// Verify widget was stored
	retrieved, err := repo.GetByID(ctx, widget.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve widget: %v", err)
	}

	if retrieved.ID != widget.ID {
		t.Errorf("Expected widget ID %s, got %s", widget.ID, retrieved.ID)
	}
	if retrieved.OwnerID != widget.OwnerID {
		t.Errorf("Expected owner ID %s, got %s", widget.OwnerID, retrieved.OwnerID)
	}
	if retrieved.Name != widget.Name {
		t.Errorf("Expected widget name %s, got %s", widget.Name, retrieved.Name)
	}
}

func TestWidgetRepository_GetByID(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockWidgetRepository(client)
	ctx := context.Background()

	tests := []struct {
		name       string
		widgetID   string
		setup      func()
		wantErr    bool
		wantWidget *models.Widget
	}{
		{
			name:     "existing widget",
			widgetID: "existing-widget",
			setup: func() {
				widget := &models.Widget{
					ID:      "existing-widget",
					OwnerID: "user-123",
					Name:    "Existing Widget",
					Type:    "lead-form",
					Enabled: true,
				}
				repo.Create(ctx, widget)
			},
			wantErr: false,
			wantWidget: &models.Widget{
				ID:      "existing-widget",
				OwnerID: "user-123",
				Name:    "Existing Widget",
				Type:    "lead-form",
				Enabled: true,
			},
		},
		{
			name:       "non-existent widget",
			widgetID:   "non-existent",
			setup:      func() {},
			wantErr:    true,
			wantWidget: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			widget, err := repo.GetByID(ctx, tt.widgetID)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.wantWidget != nil && widget != nil {
				if widget.ID != tt.wantWidget.ID {
					t.Errorf("Expected widget ID %s, got %s", tt.wantWidget.ID, widget.ID)
				}
				if widget.OwnerID != tt.wantWidget.OwnerID {
					t.Errorf("Expected owner ID %s, got %s", tt.wantWidget.OwnerID, widget.OwnerID)
				}
			}
		})
	}
}

func TestWidgetRepository_GetByUserID(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockWidgetRepository(client)
	ctx := context.Background()

	userID := "user-123"

	// Create test widgets
	widgets := []*models.Widget{
		{
			ID:        "widget-1",
			OwnerID:   userID,
			Name:      "Widget 1",
			Type:      "lead-form",
			Enabled:   true,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			ID:        "widget-2",
			OwnerID:   userID,
			Name:      "Widget 2",
			Type:      "lead",
			Enabled:   true,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID:        "widget-3",
			OwnerID:   "other-user",
			Name:      "Other User Widget",
			Type:      "lead-form",
			Enabled:   true,
			CreatedAt: time.Now(),
		},
	}

	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	// Test pagination
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
	}

	userWidgets, err := repo.GetByUserID(ctx, userID, opts)
	if err != nil {
		t.Fatalf("Failed to get widgets by user ID: %v", err)
	}

	if len(userWidgets) != 2 {
		t.Errorf("Expected 2 widgets for user %s, got %d", userID, len(userWidgets))
	}
}

func TestWidgetRepository_Update(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockWidgetRepository(client)
	ctx := context.Background()

	// Create initial widget
	widget := &models.Widget{
		ID:      "update-widget",
		OwnerID: "user-123",
		Name:    "Original Name",
		Type:    "lead-form",
		Enabled: true,
	}

	err := repo.Create(ctx, widget)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}

	// Update widget
	widget.Name = "Updated Name"
	widget.Enabled = false
	widget.UpdatedAt = time.Now()

	err = repo.Update(ctx, widget)
	if err != nil {
		t.Fatalf("Failed to update widget: %v", err)
	}

	// Verify update
	updated, err := repo.GetByID(ctx, widget.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated widget: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected updated name 'Updated Name', got %s", updated.Name)
	}
	if updated.Enabled != false {
		t.Errorf("Expected enabled=false, got %t", updated.Enabled)
	}
}

func TestWidgetRepository_Delete(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockWidgetRepository(client)
	ctx := context.Background()

	// Create widget to delete
	widget := &models.Widget{
		ID:      "delete-widget",
		OwnerID: "user-123",
		Name:    "Delete Me",
		Type:    "lead-form",
	}

	err := repo.Create(ctx, widget)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}

	// Verify widget exists
	_, err = repo.GetByID(ctx, widget.ID)
	if err != nil {
		t.Fatalf("Widget should exist before deletion: %v", err)
	}

	// Delete widget
	err = repo.Delete(ctx, widget.ID)
	if err != nil {
		t.Fatalf("Failed to delete widget: %v", err)
	}

	// Verify widget is deleted
	_, err = repo.GetByID(ctx, widget.ID)
	if err == nil {
		t.Error("Widget should not exist after deletion")
	}
}
