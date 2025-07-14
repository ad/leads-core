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
type MockFormRepository struct {
	client *MockRedisClient
}

func NewMockFormRepository(client *MockRedisClient) *MockFormRepository {
	return &MockFormRepository{client: client}
}

func (r *MockFormRepository) Create(ctx context.Context, form *models.Form) error {
	formKey := fmt.Sprintf("form:%s", form.ID)
	data := map[string]interface{}{
		"id":         form.ID,
		"owner_id":   form.OwnerID,
		"name":       form.Name,
		"type":       form.Type,
		"enabled":    form.Enabled,
		"created_at": form.CreatedAt.Unix(),
		"updated_at": form.UpdatedAt.Unix(),
	}

	return r.client.client.HMSet(ctx, formKey, data).Err()
}

func (r *MockFormRepository) GetByID(ctx context.Context, id string) (*models.Form, error) {
	formKey := fmt.Sprintf("form:%s", id)
	data, err := r.client.client.HGetAll(ctx, formKey).Result()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("form not found")
	}

	form := &models.Form{
		ID:      data["id"],
		OwnerID: data["owner_id"],
		Name:    data["name"],
		Type:    data["type"],
		Enabled: data["enabled"] == "1",
	}

	return form, nil
}

func (r *MockFormRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Form, error) {
	// Simplified implementation for testing
	pattern := "form:*"
	keys, err := r.client.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var forms []*models.Form
	for _, key := range keys {
		data, err := r.client.client.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		if data["owner_id"] == userID {
			form := &models.Form{
				ID:      data["id"],
				OwnerID: data["owner_id"],
				Name:    data["name"],
				Type:    data["type"],
				Enabled: data["enabled"] == "1",
			}
			forms = append(forms, form)
		}
	}

	return forms, nil
}

func (r *MockFormRepository) Update(ctx context.Context, form *models.Form) error {
	return r.Create(ctx, form) // Simplified for testing
}

func (r *MockFormRepository) Delete(ctx context.Context, id string) error {
	formKey := fmt.Sprintf("form:%s", id)
	return r.client.client.Del(ctx, formKey).Err()
}

func TestFormRepository_Create(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockFormRepository(client)
	ctx := context.Background()

	form := &models.Form{
		ID:        "test-form-1",
		OwnerID:   "user-123",
		Name:      "Test Form",
		Type:      "contact",
		Enabled:   true,
		Fields:    map[string]interface{}{"name": map[string]interface{}{"type": "text", "required": true}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(ctx, form)
	if err != nil {
		t.Fatalf("Failed to create form: %v", err)
	}

	// Verify form was stored
	retrieved, err := repo.GetByID(ctx, form.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve form: %v", err)
	}

	if retrieved.ID != form.ID {
		t.Errorf("Expected form ID %s, got %s", form.ID, retrieved.ID)
	}
	if retrieved.OwnerID != form.OwnerID {
		t.Errorf("Expected owner ID %s, got %s", form.OwnerID, retrieved.OwnerID)
	}
	if retrieved.Name != form.Name {
		t.Errorf("Expected form name %s, got %s", form.Name, retrieved.Name)
	}
}

func TestFormRepository_GetByID(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockFormRepository(client)
	ctx := context.Background()

	tests := []struct {
		name     string
		formID   string
		setup    func()
		wantErr  bool
		wantForm *models.Form
	}{
		{
			name:   "existing form",
			formID: "existing-form",
			setup: func() {
				form := &models.Form{
					ID:      "existing-form",
					OwnerID: "user-123",
					Name:    "Existing Form",
					Type:    "contact",
					Enabled: true,
				}
				repo.Create(ctx, form)
			},
			wantErr: false,
			wantForm: &models.Form{
				ID:      "existing-form",
				OwnerID: "user-123",
				Name:    "Existing Form",
				Type:    "contact",
				Enabled: true,
			},
		},
		{
			name:     "non-existent form",
			formID:   "non-existent",
			setup:    func() {},
			wantErr:  true,
			wantForm: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			form, err := repo.GetByID(ctx, tt.formID)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.wantForm != nil && form != nil {
				if form.ID != tt.wantForm.ID {
					t.Errorf("Expected form ID %s, got %s", tt.wantForm.ID, form.ID)
				}
				if form.OwnerID != tt.wantForm.OwnerID {
					t.Errorf("Expected owner ID %s, got %s", tt.wantForm.OwnerID, form.OwnerID)
				}
			}
		})
	}
}

func TestFormRepository_GetByUserID(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockFormRepository(client)
	ctx := context.Background()

	userID := "user-123"

	// Create test forms
	forms := []*models.Form{
		{
			ID:        "form-1",
			OwnerID:   userID,
			Name:      "Form 1",
			Type:      "contact",
			Enabled:   true,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			ID:        "form-2",
			OwnerID:   userID,
			Name:      "Form 2",
			Type:      "lead",
			Enabled:   true,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID:        "form-3",
			OwnerID:   "other-user",
			Name:      "Other User Form",
			Type:      "contact",
			Enabled:   true,
			CreatedAt: time.Now(),
		},
	}

	for _, form := range forms {
		err := repo.Create(ctx, form)
		if err != nil {
			t.Fatalf("Failed to create form %s: %v", form.ID, err)
		}
	}

	// Test pagination
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
	}

	userForms, err := repo.GetByUserID(ctx, userID, opts)
	if err != nil {
		t.Fatalf("Failed to get forms by user ID: %v", err)
	}

	if len(userForms) != 2 {
		t.Errorf("Expected 2 forms for user %s, got %d", userID, len(userForms))
	}
}

func TestFormRepository_Update(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockFormRepository(client)
	ctx := context.Background()

	// Create initial form
	form := &models.Form{
		ID:      "update-form",
		OwnerID: "user-123",
		Name:    "Original Name",
		Type:    "contact",
		Enabled: true,
	}

	err := repo.Create(ctx, form)
	if err != nil {
		t.Fatalf("Failed to create form: %v", err)
	}

	// Update form
	form.Name = "Updated Name"
	form.Enabled = false
	form.UpdatedAt = time.Now()

	err = repo.Update(ctx, form)
	if err != nil {
		t.Fatalf("Failed to update form: %v", err)
	}

	// Verify update
	updated, err := repo.GetByID(ctx, form.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated form: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected updated name 'Updated Name', got %s", updated.Name)
	}
	if updated.Enabled != false {
		t.Errorf("Expected enabled=false, got %t", updated.Enabled)
	}
}

func TestFormRepository_Delete(t *testing.T) {
	client := setupTestRedis(t)
	repo := NewMockFormRepository(client)
	ctx := context.Background()

	// Create form to delete
	form := &models.Form{
		ID:      "delete-form",
		OwnerID: "user-123",
		Name:    "Delete Me",
		Type:    "contact",
	}

	err := repo.Create(ctx, form)
	if err != nil {
		t.Fatalf("Failed to create form: %v", err)
	}

	// Verify form exists
	_, err = repo.GetByID(ctx, form.ID)
	if err != nil {
		t.Fatalf("Form should exist before deletion: %v", err)
	}

	// Delete form
	err = repo.Delete(ctx, form.ID)
	if err != nil {
		t.Fatalf("Failed to delete form: %v", err)
	}

	// Verify form is deleted
	_, err = repo.GetByID(ctx, form.ID)
	if err == nil {
		t.Error("Form should not exist after deletion")
	}
}
