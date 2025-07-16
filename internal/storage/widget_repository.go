package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/internal/errors"
	"github.com/ad/leads-core/internal/models"
	"github.com/redis/go-redis/v9"
)

// WidgetRepository defines interface for widget storage operations
type WidgetRepository interface {
	Create(ctx context.Context, widget *models.Widget) error
	GetByID(ctx context.Context, id string) (*models.Widget, error)
	GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error)
	Update(ctx context.Context, widget *models.Widget) error
	Delete(ctx context.Context, id string) error
	GetWidgetsByType(ctx context.Context, widgetType string, opts models.PaginationOptions) ([]*models.Widget, error)
	GetWidgetsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Widget, error)
}

// RedisWidgetRepository implements WidgetRepository for Redis
type RedisWidgetRepository struct {
	client    *RedisClient
	statsRepo StatsRepository
}

// NewRedisWidgetRepository creates a new Redis widget repository
func NewRedisWidgetRepository(client *RedisClient, statsRepo StatsRepository) *RedisWidgetRepository {
	return &RedisWidgetRepository{
		client:    client,
		statsRepo: statsRepo,
	}
}

// Create creates a new widget
func (r *RedisWidgetRepository) Create(ctx context.Context, widget *models.Widget) error {
	// Step 1: Store widget data and stats in the same slot using hash tag {widgetID}
	widgetSlotPipe := r.client.client.TxPipeline()

	// Store widget data
	widgetKey := GenerateWidgetKey(widget.ID)
	widgetSlotPipe.HSet(ctx, widgetKey, widget.ToRedisHash())

	// Initialize stats (same slot as widget due to {widgetID} hash tag)
	statsKey := GenerateWidgetStatsKey(widget.ID)
	widgetSlotPipe.HSet(ctx, statsKey, map[string]interface{}{
		"widget_id": widget.ID,
		"views":     0,
		"submits":   0,
		"closes":    0,
	})

	_, err := widgetSlotPipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store widget data: %w", err)
	}

	// Step 2: Update user widgets index (separate slot)
	userWidgetsKey := GenerateUserWidgetsKey(widget.OwnerID)
	timestamp := float64(widget.CreatedAt.UnixNano())
	if err := r.client.client.ZAdd(ctx, userWidgetsKey, redis.Z{Score: timestamp, Member: widget.ID}).Err(); err != nil {
		return fmt.Errorf("failed to update user widgets index: %w", err)
	}

	// Step 3: Update global indexes (separate operations to avoid cross-slot issues)
	timestamp = float64(widget.CreatedAt.Unix())
	if err := r.client.client.ZAdd(ctx, WidgetsByTimeKey, redis.Z{Score: timestamp, Member: widget.ID}).Err(); err != nil {
		return fmt.Errorf("failed to update time index: %w", err)
	}

	typeKey := GenerateWidgetsByTypeKey(widget.Type)
	if err := r.client.client.SAdd(ctx, typeKey, widget.ID).Err(); err != nil {
		return fmt.Errorf("failed to update type index: %w", err)
	}

	statusKey := GenerateWidgetsByStatusKey(widget.Enabled)
	if err := r.client.client.SAdd(ctx, statusKey, widget.ID).Err(); err != nil {
		return fmt.Errorf("failed to update status index: %w", err)
	}

	return nil
}

// GetByID retrieves a widget by ID
func (r *RedisWidgetRepository) GetByID(ctx context.Context, id string) (*models.Widget, error) {
	widgetKey := GenerateWidgetKey(id)
	hash, err := r.client.client.HGetAll(ctx, widgetKey).Result()
	if err != nil {
		return nil, err
	}

	if len(hash) == 0 {
		return nil, errors.ErrNotFound
	}

	widget := &models.Widget{}
	if err := widget.FromRedisHash(hash); err != nil {
		return nil, fmt.Errorf("failed to parse widget data: %w", err)
	}

	return widget, nil
}

// GetByUserID retrieves widgets for a specific user with pagination
func (r *RedisWidgetRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	userWidgetsKey := GenerateUserWidgetsKey(userID)

	// Get total number of widgets for the user
	total, err := r.client.client.ZCard(ctx, userWidgetsKey).Result()
	if err != nil {
		return nil, 0, err
	}

	// Calculate pagination range
	start := int64(opts.Page-1) * int64(opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	// Get widget IDs for the user, sorted by creation time (newest first)
	widgetIDs, err := r.client.client.ZRevRange(ctx, userWidgetsKey, start, end).Result()
	if err != nil {
		return nil, 0, err
	}

	if len(widgetIDs) == 0 {
		return []*models.Widget{}, int(total), nil
	}

	// Get widgets for the page
	widgets := make([]*models.Widget, 0, len(widgetIDs))

	for _, widgetID := range widgetIDs {
		widget, err := r.GetByID(ctx, widgetID)
		if err != nil {
			continue // Skip widgets that can't be loaded
		}

		// Load stats for the widget
		if stats, err := r.statsRepo.GetWidgetStats(ctx, widgetID); err == nil {
			widget.Stats = stats
		}

		widgets = append(widgets, widget)
	}

	return widgets, int(total), nil
}

// Update updates an existing widget
func (r *RedisWidgetRepository) Update(ctx context.Context, widget *models.Widget) error {
	// Get existing widget to compare indexes
	existingWidget, err := r.GetByID(ctx, widget.ID)
	if err != nil {
		return fmt.Errorf("widget not found: %w", err)
	}

	// Update widget data (atomic operation within same slot)
	widget.UpdatedAt = time.Now()
	widgetKey := GenerateWidgetKey(widget.ID)
	if err := r.client.client.HSet(ctx, widgetKey, widget.ToRedisHash()).Err(); err != nil {
		return fmt.Errorf("failed to update widget data: %w", err)
	}

	// Update indexes if necessary (separate operations)
	if existingWidget.Type != widget.Type {
		oldTypeKey := GenerateWidgetsByTypeKey(existingWidget.Type)
		newTypeKey := GenerateWidgetsByTypeKey(widget.Type)
		r.client.client.SRem(ctx, oldTypeKey, widget.ID)
		r.client.client.SAdd(ctx, newTypeKey, widget.ID)
	}

	if existingWidget.Enabled != widget.Enabled {
		oldStatusKey := GenerateWidgetsByStatusKey(existingWidget.Enabled)
		newStatusKey := GenerateWidgetsByStatusKey(widget.Enabled)
		r.client.client.SRem(ctx, oldStatusKey, widget.ID)
		r.client.client.SAdd(ctx, newStatusKey, widget.ID)
	}

	return nil
}

// Delete deletes a widget and all related data
func (r *RedisWidgetRepository) Delete(ctx context.Context, id string) error {
	// Get widget to remove from indexes
	widget, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("widget not found: %w", err)
	}

	// Step 1: Delete widget data and stats in same slot
	widgetSlotPipe := r.client.client.TxPipeline()

	widgetKey := GenerateWidgetKey(id)
	widgetSlotPipe.Del(ctx, widgetKey)

	statsKey := GenerateWidgetStatsKey(id)
	widgetSlotPipe.Del(ctx, statsKey)

	// Delete submissions in same slot
	submissionsKey := GenerateWidgetSubmissionsKey(id)
	submissionIDs, _ := r.client.client.ZRange(ctx, submissionsKey, 0, -1).Result()
	for _, submissionID := range submissionIDs {
		submissionKey := GenerateSubmissionKey(id, submissionID)
		widgetSlotPipe.Del(ctx, submissionKey)
	}
	widgetSlotPipe.Del(ctx, submissionsKey)

	_, err = widgetSlotPipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete widget data: %w", err)
	}

	// Step 2: Remove from global indexes (separate operations)
	r.client.client.ZRem(ctx, WidgetsByTimeKey, id)

	userWidgetsKey := GenerateUserWidgetsKey(widget.OwnerID)
	r.client.client.ZRem(ctx, userWidgetsKey, id)

	typeKey := GenerateWidgetsByTypeKey(widget.Type)
	r.client.client.SRem(ctx, typeKey, id)

	statusKey := GenerateWidgetsByStatusKey(widget.Enabled)
	r.client.client.SRem(ctx, statusKey, id)

	return nil
}

// GetWidgetsByType retrieves widgets by type with pagination
func (r *RedisWidgetRepository) GetWidgetsByType(ctx context.Context, widgetType string, opts models.PaginationOptions) ([]*models.Widget, error) {
	typeKey := GenerateWidgetsByTypeKey(widgetType)

	start := int64((opts.Page - 1) * opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	widgetIDs, err := r.client.client.SRandMemberN(ctx, typeKey, end-start+1).Result()
	if err != nil {
		return nil, err
	}

	widgets := make([]*models.Widget, 0, len(widgetIDs))
	for _, widgetID := range widgetIDs {
		widget, err := r.GetByID(ctx, widgetID)
		if err != nil {
			continue // Skip widgets that can't be loaded
		}

		// Load stats for the widget
		if stats, err := r.statsRepo.GetWidgetStats(ctx, widgetID); err == nil {
			widget.Stats = stats
		}

		widgets = append(widgets, widget)
	}

	return widgets, nil
}

// GetWidgetsByStatus retrieves widgets by status with pagination
func (r *RedisWidgetRepository) GetWidgetsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Widget, error) {
	statusKey := GenerateWidgetsByStatusKey(enabled)

	start := int64((opts.Page - 1) * opts.PerPage)
	end := start + int64(opts.PerPage) - 1

	widgetIDs, err := r.client.client.SRandMemberN(ctx, statusKey, end-start+1).Result()
	if err != nil {
		return nil, err
	}

	widgets := make([]*models.Widget, 0, len(widgetIDs))
	for _, widgetID := range widgetIDs {
		widget, err := r.GetByID(ctx, widgetID)
		if err != nil {
			continue // Skip widgets that can't be loaded
		}

		// Load stats for the widget
		if stats, err := r.statsRepo.GetWidgetStats(ctx, widgetID); err == nil {
			widget.Stats = stats
		}

		widgets = append(widgets, widget)
	}

	return widgets, nil
}
