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
	GetByWidgetID(ctx context.Context, widgetID string, opts models.PaginationOptions) ([]*models.Submission, int, error)
	GetByID(ctx context.Context, widgetID, submissionID string) (*models.Submission, error)
	UpdateTTL(ctx context.Context, userID string, newTTL time.Duration) error
	UpdateWidgetSubmissionsTTL(ctx context.Context, widgetID string, ttlDays int) error
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
	// All submission-related keys use {widgetID} hash tag, so they'll be in same slot
	pipe := r.client.client.TxPipeline()

	// Store submission data
	submissionKey := GenerateSubmissionKey(submission.WidgetID, submission.ID)
	pipe.HSet(ctx, submissionKey, submission.ToRedisHash())

	// Set TTL if specified
	if submission.TTL > 0 {
		pipe.Expire(ctx, submissionKey, submission.TTL)
	}

	// Add to widget submissions index (same slot due to hash tag)
	widgetSubmissionsKey := GenerateWidgetSubmissionsKey(submission.WidgetID)
	timestamp := float64(submission.CreatedAt.Unix())
	pipe.ZAdd(ctx, widgetSubmissionsKey, redis.Z{Score: timestamp, Member: submission.ID})

	_, err := pipe.Exec(ctx)
	return err
}

// GetByWidgetID retrieves submissions for a specific widget with pagination
func (r *RedisSubmissionRepository) GetByWidgetID(ctx context.Context, widgetID string, opts models.PaginationOptions) ([]*models.Submission, int, error) {
	widgetSubmissionsKey := GenerateWidgetSubmissionsKey(widgetID)

	// Get total number of submissions
	total, err := r.client.client.ZCard(ctx, widgetSubmissionsKey).Result()
	if err != nil {
		return nil, 0, err
	}

	// Calculate pagination range
	start := int64((opts.Page - 1) * opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	// Get submission IDs (sorted by timestamp, newest first)
	submissionIDs, err := r.client.client.ZRevRange(ctx, widgetSubmissionsKey, start, end).Result()
	if err != nil {
		return nil, 0, err
	}

	if len(submissionIDs) == 0 {
		return []*models.Submission{}, 0, nil
	}

	// Get submissions
	submissions := make([]*models.Submission, 0, len(submissionIDs))
	for _, submissionID := range submissionIDs {
		submission, err := r.GetByID(ctx, widgetID, submissionID)
		if err != nil {
			continue // Skip submissions that can't be loaded (expired, etc.)
		}
		submissions = append(submissions, submission)
	}

	return submissions, int(total), nil
}

// GetByID retrieves a specific submission
func (r *RedisSubmissionRepository) GetByID(ctx context.Context, widgetID, submissionID string) (*models.Submission, error) {
	submissionKey := GenerateSubmissionKey(widgetID, submissionID)
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
	// Get all widgets for the user
	userWidgetsKey := GenerateUserWidgetsKey(userID)
	widgetIDs, err := r.client.client.SMembers(ctx, userWidgetsKey).Result()
	if err != nil {
		return err
	}

	pipe := r.client.client.TxPipeline()

	// Update TTL for all submissions of each widget
	for _, widgetID := range widgetIDs {
		widgetSubmissionsKey := GenerateWidgetSubmissionsKey(widgetID)
		submissionIDs, err := r.client.client.ZRange(ctx, widgetSubmissionsKey, 0, -1).Result()
		if err != nil {
			continue // Skip this widget if we can't get submissions
		}

		for _, submissionID := range submissionIDs {
			submissionKey := GenerateSubmissionKey(widgetID, submissionID)
			pipe.Expire(ctx, submissionKey, newTTL)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// UpdateWidgetSubmissionsTTL updates TTL for all submissions of a specific widget
func (r *RedisSubmissionRepository) UpdateWidgetSubmissionsTTL(ctx context.Context, widgetID string, ttlDays int) error {
	pipe := r.client.client.Pipeline()

	// Get all submissions for the widget
	submissionsKey := fmt.Sprintf("widget:%s:submissions", widgetID)
	submissionIDs, err := r.client.client.ZRange(ctx, submissionsKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get submissions for widget %s: %w", widgetID, err)
	}

	// Update TTL for each submission
	ttlDuration := time.Duration(ttlDays) * 24 * time.Hour
	for _, submissionID := range submissionIDs {
		submissionKey := fmt.Sprintf("submission:%s:%s", widgetID, submissionID)
		pipe.Expire(ctx, submissionKey, ttlDuration)
	}

	// Update TTL for the submissions list itself
	pipe.Expire(ctx, submissionsKey, ttlDuration)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update TTL for widget %s submissions: %w", widgetID, err)
	}

	return nil
}
