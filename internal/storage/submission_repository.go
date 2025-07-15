package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/redis/go-redis/v9"
)

// SubmissionRepository defines interface for submission storage operations
type SubmissionRepository interface {
	Create(ctx context.Context, submission *models.Submission) error
	GetByFormID(ctx context.Context, formID string, opts models.PaginationOptions) ([]*models.Submission, error)
	GetByID(ctx context.Context, formID, submissionID string) (*models.Submission, error)
	UpdateTTL(ctx context.Context, userID string, newTTL time.Duration) error
	UpdateFormSubmissionsTTL(ctx context.Context, formID string, ttlDays int) error
}

// RedisSubmissionRepository implements SubmissionRepository for Redis
type RedisSubmissionRepository struct {
	client *RedisClient
}

// NewRedisSubmissionRepository creates a new Redis submission repository
func NewRedisSubmissionRepository(client *RedisClient) *RedisSubmissionRepository {
	return &RedisSubmissionRepository{client: client}
}

// Create creates a new submission with TTL
func (r *RedisSubmissionRepository) Create(ctx context.Context, submission *models.Submission) error {
	// All submission-related keys use {formID} hash tag, so they'll be in same slot
	pipe := r.client.client.TxPipeline()

	// Store submission data
	submissionKey := GenerateSubmissionKey(submission.FormID, submission.ID)
	pipe.HSet(ctx, submissionKey, submission.ToRedisHash())

	// Set TTL if specified
	if submission.TTL > 0 {
		pipe.Expire(ctx, submissionKey, submission.TTL)
	}

	// Add to form submissions index (same slot due to hash tag)
	formSubmissionsKey := GenerateFormSubmissionsKey(submission.FormID)
	timestamp := float64(submission.CreatedAt.Unix())
	pipe.ZAdd(ctx, formSubmissionsKey, redis.Z{Score: timestamp, Member: submission.ID})

	_, err := pipe.Exec(ctx)
	return err
}

// GetByFormID retrieves submissions for a specific form with pagination
func (r *RedisSubmissionRepository) GetByFormID(ctx context.Context, formID string, opts models.PaginationOptions) ([]*models.Submission, error) {
	formSubmissionsKey := GenerateFormSubmissionsKey(formID)

	// Calculate pagination range
	start := int64(opts.Page * opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	// Get submission IDs (sorted by timestamp, newest first)
	submissionIDs, err := r.client.client.ZRevRange(ctx, formSubmissionsKey, start, end).Result()
	if err != nil {
		return nil, err
	}

	if len(submissionIDs) == 0 {
		return []*models.Submission{}, nil
	}

	// Get submissions
	submissions := make([]*models.Submission, 0, len(submissionIDs))
	for _, submissionID := range submissionIDs {
		submission, err := r.GetByID(ctx, formID, submissionID)
		if err != nil {
			continue // Skip submissions that can't be loaded (expired, etc.)
		}
		submissions = append(submissions, submission)
	}

	return submissions, nil
}

// GetByID retrieves a specific submission
func (r *RedisSubmissionRepository) GetByID(ctx context.Context, formID, submissionID string) (*models.Submission, error) {
	submissionKey := GenerateSubmissionKey(formID, submissionID)
	hash, err := r.client.client.HGetAll(ctx, submissionKey).Result()
	if err != nil {
		return nil, err
	}

	if len(hash) == 0 {
		return nil, fmt.Errorf("submission not found")
	}

	submission := &models.Submission{}
	if err := submission.FromRedisHash(hash); err != nil {
		return nil, fmt.Errorf("failed to parse submission data: %w", err)
	}

	return submission, nil
}

// UpdateTTL updates TTL for all submissions of a user
func (r *RedisSubmissionRepository) UpdateTTL(ctx context.Context, userID string, newTTL time.Duration) error {
	// Get all forms for the user
	userFormsKey := GenerateUserFormsKey(userID)
	formIDs, err := r.client.client.SMembers(ctx, userFormsKey).Result()
	if err != nil {
		return err
	}

	pipe := r.client.client.TxPipeline()

	// Update TTL for all submissions of each form
	for _, formID := range formIDs {
		formSubmissionsKey := GenerateFormSubmissionsKey(formID)
		submissionIDs, err := r.client.client.ZRange(ctx, formSubmissionsKey, 0, -1).Result()
		if err != nil {
			continue // Skip this form if we can't get submissions
		}

		for _, submissionID := range submissionIDs {
			submissionKey := GenerateSubmissionKey(formID, submissionID)
			pipe.Expire(ctx, submissionKey, newTTL)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// UpdateFormSubmissionsTTL updates TTL for all submissions of a specific form
func (r *RedisSubmissionRepository) UpdateFormSubmissionsTTL(ctx context.Context, formID string, ttlDays int) error {
	pipe := r.client.client.Pipeline()

	// Get all submissions for the form
	submissionsKey := fmt.Sprintf("form:%s:submissions", formID)
	submissionIDs, err := r.client.client.ZRange(ctx, submissionsKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get submissions for form %s: %w", formID, err)
	}

	// Update TTL for each submission
	ttlDuration := time.Duration(ttlDays) * 24 * time.Hour
	for _, submissionID := range submissionIDs {
		submissionKey := fmt.Sprintf("submission:%s:%s", formID, submissionID)
		pipe.Expire(ctx, submissionKey, ttlDuration)
	}

	// Update TTL for the submissions list itself
	pipe.Expire(ctx, submissionsKey, ttlDuration)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update TTL for form %s submissions: %w", formID, err)
	}

	return nil
}
