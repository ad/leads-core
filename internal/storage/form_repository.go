package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/redis/go-redis/v9"
)

// FormRepository defines interface for form storage operations
type FormRepository interface {
	Create(ctx context.Context, form *models.Form) error
	GetByID(ctx context.Context, id string) (*models.Form, error)
	GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Form, error)
	Update(ctx context.Context, form *models.Form) error
	Delete(ctx context.Context, id string) error
	GetFormsByType(ctx context.Context, formType string, opts models.PaginationOptions) ([]*models.Form, error)
	GetFormsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Form, error)
}

// RedisFormRepository implements FormRepository for Redis
type RedisFormRepository struct {
	client *RedisClient
}

// NewRedisFormRepository creates a new Redis form repository
func NewRedisFormRepository(client *RedisClient) *RedisFormRepository {
	return &RedisFormRepository{client: client}
}

// Create creates a new form
func (r *RedisFormRepository) Create(ctx context.Context, form *models.Form) error {
	pipe := r.client.client.TxPipeline()

	// Store form data
	formKey := GenerateFormKey(form.ID)
	pipe.HSet(ctx, formKey, form.ToRedisHash())

	// Add to indexes
	timestamp := float64(form.CreatedAt.Unix())
	pipe.ZAdd(ctx, FormsByTimeKey, redis.Z{Score: timestamp, Member: form.ID})

	userFormsKey := GenerateUserFormsKey(form.OwnerID)
	pipe.SAdd(ctx, userFormsKey, form.ID)

	typeKey := GenerateFormsByTypeKey(form.Type)
	pipe.SAdd(ctx, typeKey, form.ID)

	statusKey := GenerateFormsByStatusKey(form.Enabled)
	pipe.SAdd(ctx, statusKey, form.ID)

	// Initialize stats
	statsKey := GenerateFormStatsKey(form.ID)
	pipe.HSet(ctx, statsKey, map[string]interface{}{
		"form_id": form.ID,
		"views":   0,
		"submits": 0,
		"closes":  0,
	})

	_, err := pipe.Exec(ctx)
	return err
}

// GetByID retrieves a form by ID
func (r *RedisFormRepository) GetByID(ctx context.Context, id string) (*models.Form, error) {
	formKey := GenerateFormKey(id)
	hash, err := r.client.client.HGetAll(ctx, formKey).Result()
	if err != nil {
		return nil, err
	}

	if len(hash) == 0 {
		return nil, fmt.Errorf("form not found")
	}

	form := &models.Form{}
	if err := form.FromRedisHash(hash); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	return form, nil
}

// GetByUserID retrieves forms for a specific user with pagination
func (r *RedisFormRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Form, error) {
	userFormsKey := GenerateUserFormsKey(userID)

	// Get form IDs for the user
	formIDs, err := r.client.client.SMembers(ctx, userFormsKey).Result()
	if err != nil {
		return nil, err
	}

	if len(formIDs) == 0 {
		return []*models.Form{}, nil
	}

	// Calculate pagination
	start := opts.Page * opts.PerPage
	end := start + opts.PerPage
	if start >= len(formIDs) {
		return []*models.Form{}, nil
	}
	if end > len(formIDs) {
		end = len(formIDs)
	}

	// Get forms for the page
	paginatedIDs := formIDs[start:end]
	forms := make([]*models.Form, 0, len(paginatedIDs))

	for _, formID := range paginatedIDs {
		form, err := r.GetByID(ctx, formID)
		if err != nil {
			continue // Skip forms that can't be loaded
		}
		forms = append(forms, form)
	}

	return forms, nil
}

// Update updates an existing form
func (r *RedisFormRepository) Update(ctx context.Context, form *models.Form) error {
	// Get existing form to compare indexes
	existingForm, err := r.GetByID(ctx, form.ID)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	pipe := r.client.client.TxPipeline()

	// Update form data
	form.UpdatedAt = time.Now()
	formKey := GenerateFormKey(form.ID)
	pipe.HSet(ctx, formKey, form.ToRedisHash())

	// Update indexes if necessary
	if existingForm.Type != form.Type {
		oldTypeKey := GenerateFormsByTypeKey(existingForm.Type)
		newTypeKey := GenerateFormsByTypeKey(form.Type)
		pipe.SRem(ctx, oldTypeKey, form.ID)
		pipe.SAdd(ctx, newTypeKey, form.ID)
	}

	if existingForm.Enabled != form.Enabled {
		oldStatusKey := GenerateFormsByStatusKey(existingForm.Enabled)
		newStatusKey := GenerateFormsByStatusKey(form.Enabled)
		pipe.SRem(ctx, oldStatusKey, form.ID)
		pipe.SAdd(ctx, newStatusKey, form.ID)
	}

	_, err = pipe.Exec(ctx)
	return err
}

// Delete deletes a form and all related data
func (r *RedisFormRepository) Delete(ctx context.Context, id string) error {
	// Get form to remove from indexes
	form, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	pipe := r.client.client.TxPipeline()

	// Delete form data
	formKey := GenerateFormKey(id)
	pipe.Del(ctx, formKey)

	// Remove from indexes
	pipe.ZRem(ctx, FormsByTimeKey, id)

	userFormsKey := GenerateUserFormsKey(form.OwnerID)
	pipe.SRem(ctx, userFormsKey, id)

	typeKey := GenerateFormsByTypeKey(form.Type)
	pipe.SRem(ctx, typeKey, id)

	statusKey := GenerateFormsByStatusKey(form.Enabled)
	pipe.SRem(ctx, statusKey, id)

	// Delete stats
	statsKey := GenerateFormStatsKey(id)
	pipe.Del(ctx, statsKey)

	// Delete all submissions for this form
	submissionsKey := GenerateFormSubmissionsKey(id)
	submissionIDs, _ := r.client.client.ZRange(ctx, submissionsKey, 0, -1).Result()
	for _, submissionID := range submissionIDs {
		submissionKey := GenerateSubmissionKey(id, submissionID)
		pipe.Del(ctx, submissionKey)
	}
	pipe.Del(ctx, submissionsKey)

	_, err = pipe.Exec(ctx)
	return err
}

// GetFormsByType retrieves forms by type with pagination
func (r *RedisFormRepository) GetFormsByType(ctx context.Context, formType string, opts models.PaginationOptions) ([]*models.Form, error) {
	typeKey := GenerateFormsByTypeKey(formType)

	start := int64(opts.Page * opts.PerPage)
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
		forms = append(forms, form)
	}

	return forms, nil
}

// GetFormsByStatus retrieves forms by status with pagination
func (r *RedisFormRepository) GetFormsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Form, error) {
	statusKey := GenerateFormsByStatusKey(enabled)

	start := int64(opts.Page * opts.PerPage)
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
		forms = append(forms, form)
	}

	return forms, nil
}
