package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	GetByUserIDWithFilters(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error)
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

	statusKey := GenerateWidgetsByStatusKey(widget.IsVisible)
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

	// Get widgets for the page using batch operations
	widgets, err := r.batchLoadWidgets(ctx, widgetIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to batch load widgets: %w", err)
	}

	return widgets, int(total), nil
}

// GetByUserIDWithFilters retrieves widgets for a specific user with filtering and pagination
func (r *RedisWidgetRepository) GetByUserIDWithFilters(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	// If no filters are applied, use the existing method for optimal performance
	if opts.Filters == nil || !opts.Filters.HasFilters() {
		return r.GetByUserID(ctx, userID, opts)
	}

	// Validate and clean filter options
	filters := models.ValidateFilterOptions(opts.Filters)
	if filters == nil || !filters.HasFilters() {
		return r.GetByUserID(ctx, userID, opts)
	}

	// Get filtered widget IDs using Redis set operations
	filteredWidgetIDs, err := r.getFilteredWidgetIDs(ctx, userID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get filtered widget IDs: %w", err)
	}

	// Apply name search filter if specified
	if filters.HasSearchFilter() {
		filteredWidgetIDs, err = r.applyNameSearchFilter(ctx, filteredWidgetIDs, filters.Search)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to apply name search filter: %w", err)
		}
	}

	total := len(filteredWidgetIDs)

	// Apply pagination to filtered results
	start := (opts.Page - 1) * opts.PerPage
	end := start + opts.PerPage

	if start >= total {
		return []*models.Widget{}, total, nil
	}

	if end > total {
		end = total
	}

	paginatedWidgetIDs := filteredWidgetIDs[start:end]

	// Load widgets for the current page using batch operations
	widgets, err := r.batchLoadWidgets(ctx, paginatedWidgetIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to batch load widgets: %w", err)
	}

	return widgets, total, nil
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

	if existingWidget.IsVisible != widget.IsVisible {
		oldStatusKey := GenerateWidgetsByStatusKey(existingWidget.IsVisible)
		newStatusKey := GenerateWidgetsByStatusKey(widget.IsVisible)
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

	statusKey := GenerateWidgetsByStatusKey(widget.IsVisible)
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

// getFilteredWidgetIDs applies Redis-based filtering to get widget IDs
func (r *RedisWidgetRepository) getFilteredWidgetIDs(ctx context.Context, userID string, filters *models.FilterOptions) ([]string, error) {
	userWidgetsKey := GenerateUserWidgetsKey(userID)

	// Build list of sets to intersect
	setsToIntersect := []string{userWidgetsKey}

	// Add type filter sets
	if filters.HasTypeFilter() {
		for _, widgetType := range filters.Types {
			typeKey := GenerateWidgetsByTypeKey(widgetType)
			setsToIntersect = append(setsToIntersect, typeKey)
		}
	}

	// Add visibility filter set
	if filters.HasVisibilityFilter() {
		statusKey := GenerateWidgetsByStatusKey(*filters.IsVisible)
		setsToIntersect = append(setsToIntersect, statusKey)
	}

	// If we only have user widgets (no additional filters), get all user widgets
	if len(setsToIntersect) == 1 {
		// Get all user widgets sorted by creation time (newest first)
		widgetIDs, err := r.client.client.ZRevRange(ctx, userWidgetsKey, 0, -1).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get user widgets: %w", err)
		}
		return widgetIDs, nil
	}

	// Use SINTER to find intersection of all sets
	// Note: We need to convert the user widgets ZSET to a SET for intersection
	// Note: For type filters with multiple types, we need to handle them differently
	if filters.HasTypeFilter() && len(filters.Types) > 1 {
		return r.getFilteredWidgetIDsWithMultipleTypes(ctx, userID, filters)
	}

	// Create a temporary SET from user widgets ZSET for intersection
	tempUserSetKey := fmt.Sprintf("temp:user_set:%s:%d", userID, time.Now().UnixNano())
	defer r.client.client.Del(ctx, tempUserSetKey) // Clean up temp key

	// Get all user widget IDs and add them to a temporary SET
	userWidgetIDs, err := r.client.client.ZRange(ctx, userWidgetsKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user widgets: %w", err)
	}

	if len(userWidgetIDs) == 0 {
		return []string{}, nil
	}

	// Add user widgets to temporary SET
	if err := r.client.client.SAdd(ctx, tempUserSetKey, userWidgetIDs).Err(); err != nil {
		return nil, fmt.Errorf("failed to create temporary user set: %w", err)
	}

	// Replace userWidgetsKey with tempUserSetKey in the intersection
	setsToIntersect[0] = tempUserSetKey

	// Single type or visibility filter - use direct SINTER
	widgetIDs, err := r.client.client.SInter(ctx, setsToIntersect...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to intersect widget sets: %w", err)
	}

	// Sort by creation time (newest first) by getting scores from user widgets zset
	return r.sortWidgetIDsByCreationTime(ctx, userWidgetsKey, widgetIDs)
}

// getFilteredWidgetIDsWithMultipleTypes handles filtering when multiple types are specified
func (r *RedisWidgetRepository) getFilteredWidgetIDsWithMultipleTypes(ctx context.Context, userID string, filters *models.FilterOptions) ([]string, error) {
	userWidgetsKey := GenerateUserWidgetsKey(userID)

	// Create union of all type sets first
	typeKeys := make([]string, len(filters.Types))
	for i, widgetType := range filters.Types {
		typeKeys[i] = GenerateWidgetsByTypeKey(widgetType)
	}

	// Use SUNION to get all widgets of specified types
	typeUnionKey := fmt.Sprintf("temp:type_union:%s:%d", userID, time.Now().UnixNano())
	defer r.client.client.Del(ctx, typeUnionKey) // Clean up temp key

	if err := r.client.client.SUnionStore(ctx, typeUnionKey, typeKeys...).Err(); err != nil {
		return nil, fmt.Errorf("failed to create type union: %w", err)
	}

	// Create a temporary SET from user widgets ZSET for intersection
	tempUserSetKey := fmt.Sprintf("temp:user_set:%s:%d", userID, time.Now().UnixNano())
	defer r.client.client.Del(ctx, tempUserSetKey) // Clean up temp key

	// Get all user widget IDs and add them to a temporary SET
	userWidgetIDs, err := r.client.client.ZRange(ctx, userWidgetsKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user widgets: %w", err)
	}

	if len(userWidgetIDs) == 0 {
		return []string{}, nil
	}

	// Add user widgets to temporary SET
	if err := r.client.client.SAdd(ctx, tempUserSetKey, userWidgetIDs).Err(); err != nil {
		return nil, fmt.Errorf("failed to create temporary user set: %w", err)
	}

	// Now intersect user widgets with type union and visibility filter if present
	setsToIntersect := []string{tempUserSetKey, typeUnionKey}

	if filters.HasVisibilityFilter() {
		statusKey := GenerateWidgetsByStatusKey(*filters.IsVisible)
		setsToIntersect = append(setsToIntersect, statusKey)
	}

	widgetIDs, err := r.client.client.SInter(ctx, setsToIntersect...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to intersect widget sets with multiple types: %w", err)
	}

	// Sort by creation time (newest first)
	return r.sortWidgetIDsByCreationTime(ctx, userWidgetsKey, widgetIDs)
}

// sortWidgetIDsByCreationTime sorts widget IDs by their creation time from user widgets zset
func (r *RedisWidgetRepository) sortWidgetIDsByCreationTime(ctx context.Context, userWidgetsKey string, widgetIDs []string) ([]string, error) {
	if len(widgetIDs) == 0 {
		return widgetIDs, nil
	}

	// Get scores (timestamps) for all widget IDs
	pipe := r.client.client.Pipeline()
	scoreCommands := make([]*redis.FloatCmd, len(widgetIDs))

	for i, widgetID := range widgetIDs {
		scoreCommands[i] = pipe.ZScore(ctx, userWidgetsKey, widgetID)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get widget scores: %w", err)
	}

	// Create slice of widget ID and timestamp pairs
	type widgetWithTime struct {
		id        string
		timestamp float64
	}

	widgetsWithTime := make([]widgetWithTime, 0, len(widgetIDs))
	for i, widgetID := range widgetIDs {
		score, err := scoreCommands[i].Result()
		if err != nil {
			// If we can't get the score, use current time (will be sorted last)
			score = float64(time.Now().UnixNano())
		}
		widgetsWithTime = append(widgetsWithTime, widgetWithTime{
			id:        widgetID,
			timestamp: score,
		})
	}

	// Sort by timestamp descending (newest first)
	for i := 0; i < len(widgetsWithTime)-1; i++ {
		for j := i + 1; j < len(widgetsWithTime); j++ {
			if widgetsWithTime[i].timestamp < widgetsWithTime[j].timestamp {
				widgetsWithTime[i], widgetsWithTime[j] = widgetsWithTime[j], widgetsWithTime[i]
			}
		}
	}

	// Extract sorted widget IDs
	sortedWidgetIDs := make([]string, len(widgetsWithTime))
	for i, widget := range widgetsWithTime {
		sortedWidgetIDs[i] = widget.id
	}

	return sortedWidgetIDs, nil
}

// applyNameSearchFilter filters widget IDs by name search using batch operations
func (r *RedisWidgetRepository) applyNameSearchFilter(ctx context.Context, widgetIDs []string, searchTerm string) ([]string, error) {
	if len(widgetIDs) == 0 || searchTerm == "" {
		return widgetIDs, nil
	}

	// Convert search term to lowercase for case-insensitive search
	searchLower := strings.ToLower(searchTerm)

	// Batch load widget names using pipeline for better performance
	pipe := r.client.client.Pipeline()
	nameCommands := make([]*redis.StringCmd, len(widgetIDs))

	for i, widgetID := range widgetIDs {
		widgetKey := GenerateWidgetKey(widgetID)
		nameCommands[i] = pipe.HGet(ctx, widgetKey, "name")
	}

	// Execute all commands at once
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to batch load widget names: %w", err)
	}

	// Filter based on names with pre-allocated slice
	filteredIDs := make([]string, 0, len(widgetIDs)/4) // Estimate 25% match rate
	for i, widgetID := range widgetIDs {
		name, err := nameCommands[i].Result()
		if err != nil {
			continue // Skip widgets that can't be loaded
		}

		// Case-insensitive name search with optimized string comparison
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, searchLower) {
			filteredIDs = append(filteredIDs, widgetID)
		}
	}

	return filteredIDs, nil
}

// batchLoadWidgets loads multiple widgets efficiently using pipeline operations
func (r *RedisWidgetRepository) batchLoadWidgets(ctx context.Context, widgetIDs []string) ([]*models.Widget, error) {
	if len(widgetIDs) == 0 {
		return []*models.Widget{}, nil
	}

	// Batch load widget data using pipeline
	pipe := r.client.client.Pipeline()
	widgetCommands := make([]*redis.MapStringStringCmd, len(widgetIDs))
	statsCommands := make([]*redis.MapStringStringCmd, len(widgetIDs))

	for i, widgetID := range widgetIDs {
		widgetKey := GenerateWidgetKey(widgetID)
		statsKey := GenerateWidgetStatsKey(widgetID)

		widgetCommands[i] = pipe.HGetAll(ctx, widgetKey)
		statsCommands[i] = pipe.HGetAll(ctx, statsKey)
	}

	// Execute all commands at once
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to batch load widget data: %w", err)
	}

	// Parse results
	widgets := make([]*models.Widget, 0, len(widgetIDs))
	for i, widgetID := range widgetIDs {
		// Parse widget data
		widgetHash, err := widgetCommands[i].Result()
		if err != nil || len(widgetHash) == 0 {
			continue // Skip widgets that can't be loaded
		}

		widget := &models.Widget{}
		if err := widget.FromRedisHash(widgetHash); err != nil {
			continue // Skip widgets with invalid data
		}

		// Parse stats data
		statsHash, err := statsCommands[i].Result()
		if err == nil && len(statsHash) > 0 {
			stats := &models.WidgetStats{WidgetID: widgetID}

			if viewsStr, ok := statsHash["views"]; ok {
				if views, err := strconv.ParseInt(viewsStr, 10, 64); err == nil {
					stats.Views = views
				}
			}

			if submitsStr, ok := statsHash["submits"]; ok {
				if submits, err := strconv.ParseInt(submitsStr, 10, 64); err == nil {
					stats.Submits = submits
				}
			}

			if closesStr, ok := statsHash["closes"]; ok {
				if closes, err := strconv.ParseInt(closesStr, 10, 64); err == nil {
					stats.Closes = closes
				}
			}

			if lastViewStr, ok := statsHash["last_view"]; ok {
				if timestamp, err := strconv.ParseInt(lastViewStr, 10, 64); err == nil {
					stats.LastView = time.Unix(timestamp, 0)
				}
			}

			widget.Stats = stats
		}

		widgets = append(widgets, widget)
	}

	return widgets, nil
}
