package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/internal/errors"
	"github.com/ad/leads-core/internal/models"
	"github.com/redis/go-redis/v9"
)

// FormRepository defines interface for form storage operations
type FormRepository interface {
	Create(ctx context.Context, form *models.Form) error
	GetByID(ctx context.Context, id string) (*models.Form, error)
	GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Form, int, error)
	Update(ctx context.Context, form *models.Form) error
	Delete(ctx context.Context, id string) error
	GetFormsByType(ctx context.Context, formType string, opts models.PaginationOptions) ([]*models.Form, error)
	GetFormsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Form, error)
}

// RedisFormRepository implements FormRepository for Redis
type RedisFormRepository struct {
	client    *RedisClient
	statsRepo StatsRepository
}

// NewRedisFormRepository creates a new Redis form repository
func NewRedisFormRepository(client *RedisClient, statsRepo StatsRepository) *RedisFormRepository {
	return &RedisFormRepository{
		client:    client,
		statsRepo: statsRepo,
	}
}

// Create creates a new form
func (r *RedisFormRepository) Create(ctx context.Context, form *models.Form) error {
	// Step 1: Store form data and stats in the same slot using hash tag {formID}
	formSlotPipe := r.client.client.TxPipeline()

	// Store form data
	formKey := GenerateFormKey(form.ID)
	formSlotPipe.HSet(ctx, formKey, form.ToRedisHash())

	// Initialize stats (same slot as form due to {formID} hash tag)
	statsKey := GenerateFormStatsKey(form.ID)
	formSlotPipe.HSet(ctx, statsKey, map[string]interface{}{
		"form_id": form.ID,
		"views":   0,
		"submits": 0,
		"closes":  0,
	})

	_, err := formSlotPipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store form data: %w", err)
	}

	// Step 2: Update user forms index (separate slot)
	userFormsKey := GenerateUserFormsKey(form.OwnerID)
	timestamp := float64(form.CreatedAt.UnixNano())
	if err := r.client.client.ZAdd(ctx, userFormsKey, redis.Z{Score: timestamp, Member: form.ID}).Err(); err != nil {
		return fmt.Errorf("failed to update user forms index: %w", err)
	}

	// Step 3: Update global indexes (separate operations to avoid cross-slot issues)
	timestamp = float64(form.CreatedAt.Unix())
	if err := r.client.client.ZAdd(ctx, FormsByTimeKey, redis.Z{Score: timestamp, Member: form.ID}).Err(); err != nil {
		return fmt.Errorf("failed to update time index: %w", err)
	}

	typeKey := GenerateFormsByTypeKey(form.Type)
	if err := r.client.client.SAdd(ctx, typeKey, form.ID).Err(); err != nil {
		return fmt.Errorf("failed to update type index: %w", err)
	}

	statusKey := GenerateFormsByStatusKey(form.Enabled)
	if err := r.client.client.SAdd(ctx, statusKey, form.ID).Err(); err != nil {
		return fmt.Errorf("failed to update status index: %w", err)
	}

	return nil
}

// GetByID retrieves a form by ID
func (r *RedisFormRepository) GetByID(ctx context.Context, id string) (*models.Form, error) {
	formKey := GenerateFormKey(id)
	hash, err := r.client.client.HGetAll(ctx, formKey).Result()
	if err != nil {
		return nil, err
	}

	if len(hash) == 0 {
		return nil, errors.ErrNotFound
	}

	form := &models.Form{}
	if err := form.FromRedisHash(hash); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	return form, nil
}

// GetByUserID retrieves forms for a specific user with pagination
func (r *RedisFormRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Form, int, error) {
	userFormsKey := GenerateUserFormsKey(userID)

	// Get total number of forms for the user
	total, err := r.client.client.ZCard(ctx, userFormsKey).Result()
	if err != nil {
		return nil, 0, err
	}

	// Calculate pagination range
	start := int64(opts.Page-1) * int64(opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	// Get form IDs for the user, sorted by creation time (newest first)
	formIDs, err := r.client.client.ZRevRange(ctx, userFormsKey, start, end).Result()
	if err != nil {
		return nil, 0, err
	}

	if len(formIDs) == 0 {
		return []*models.Form{}, int(total), nil
	}

	// Get forms for the page
	forms := make([]*models.Form, 0, len(formIDs))

	for _, formID := range formIDs {
		form, err := r.GetByID(ctx, formID)
		if err != nil {
			continue // Skip forms that can't be loaded
		}

		// Load stats for the form
		if stats, err := r.statsRepo.GetFormStats(ctx, formID); err == nil {
			form.Stats = stats
		}

		forms = append(forms, form)
	}

	return forms, int(total), nil
}

// Update updates an existing form
func (r *RedisFormRepository) Update(ctx context.Context, form *models.Form) error {
	// Get existing form to compare indexes
	existingForm, err := r.GetByID(ctx, form.ID)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	// Update form data (atomic operation within same slot)
	form.UpdatedAt = time.Now()
	formKey := GenerateFormKey(form.ID)
	if err := r.client.client.HSet(ctx, formKey, form.ToRedisHash()).Err(); err != nil {
		return fmt.Errorf("failed to update form data: %w", err)
	}

	// Update indexes if necessary (separate operations)
	if existingForm.Type != form.Type {
		oldTypeKey := GenerateFormsByTypeKey(existingForm.Type)
		newTypeKey := GenerateFormsByTypeKey(form.Type)
		r.client.client.SRem(ctx, oldTypeKey, form.ID)
		r.client.client.SAdd(ctx, newTypeKey, form.ID)
	}

	if existingForm.Enabled != form.Enabled {
		oldStatusKey := GenerateFormsByStatusKey(existingForm.Enabled)
		newStatusKey := GenerateFormsByStatusKey(form.Enabled)
		r.client.client.SRem(ctx, oldStatusKey, form.ID)
		r.client.client.SAdd(ctx, newStatusKey, form.ID)
	}

	return nil
}

// Delete deletes a form and all related data
func (r *RedisFormRepository) Delete(ctx context.Context, id string) error {
	// Get form to remove from indexes
	form, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	// Step 1: Delete form data and stats in same slot
	formSlotPipe := r.client.client.TxPipeline()

	formKey := GenerateFormKey(id)
	formSlotPipe.Del(ctx, formKey)

	statsKey := GenerateFormStatsKey(id)
	formSlotPipe.Del(ctx, statsKey)

	// Delete submissions in same slot
	submissionsKey := GenerateFormSubmissionsKey(id)
	submissionIDs, _ := r.client.client.ZRange(ctx, submissionsKey, 0, -1).Result()
	for _, submissionID := range submissionIDs {
		submissionKey := GenerateSubmissionKey(id, submissionID)
		formSlotPipe.Del(ctx, submissionKey)
	}
	formSlotPipe.Del(ctx, submissionsKey)

	_, err = formSlotPipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete form data: %w", err)
	}

	// Step 2: Remove from global indexes (separate operations)
	r.client.client.ZRem(ctx, FormsByTimeKey, id)

	userFormsKey := GenerateUserFormsKey(form.OwnerID)
	r.client.client.ZRem(ctx, userFormsKey, id)

	typeKey := GenerateFormsByTypeKey(form.Type)
	r.client.client.SRem(ctx, typeKey, id)

	statusKey := GenerateFormsByStatusKey(form.Enabled)
	r.client.client.SRem(ctx, statusKey, id)

	return nil
}

// GetFormsByType retrieves forms by type with pagination
func (r *RedisFormRepository) GetFormsByType(ctx context.Context, formType string, opts models.PaginationOptions) ([]*models.Form, error) {
	typeKey := GenerateFormsByTypeKey(formType)

	start := int64((opts.Page - 1) * opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	formIDs, err := r.client.client.SRandMemberN(ctx, typeKey, end-start+1).Result()
	if err != nil {
		return nil, err
	}

	forms := make([]*models.Form, 0, len(formIDs))
	for _, formID := range formIDs {
		form, err := r.GetByID(ctx, formID)
		if err != nil {
			continue // Skip forms that can't be loaded
		}

		// Load stats for the form
		if stats, err := r.statsRepo.GetFormStats(ctx, formID); err == nil {
			form.Stats = stats
		}

		forms = append(forms, form)
	}

	return forms, nil
}

// GetFormsByStatus retrieves forms by status with pagination
func (r *RedisFormRepository) GetFormsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Form, error) {
	statusKey := GenerateFormsByStatusKey(enabled)

	start := int64((opts.Page - 1) * opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	formIDs, err := r.client.client.SRandMemberN(ctx, statusKey, end-start+1).Result()
	if err != nil {
		return nil, err
	}

	forms := make([]*models.Form, 0, len(formIDs))
	for _, formID := range formIDs {
		form, err := r.GetByID(ctx, formID)
		if err != nil {
			continue // Skip forms that can't be loaded
		}

		// Load stats for the form
		if stats, err := r.statsRepo.GetFormStats(ctx, formID); err == nil {
			form.Stats = stats
		}

		forms = append(forms, form)
	}

	return forms, nil
}
