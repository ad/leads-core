package storage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/redis/go-redis/v9"
)

// widgetWithTime holds widget ID and creation time for sorting
type widgetWithTime struct {
	id   string
	time float64
}

// OptimizedWidgetRepository provides additional optimizations for widget operations
type OptimizedWidgetRepository struct {
	*RedisWidgetRepository
	typeCache *WidgetTypeCache
}

// NewOptimizedWidgetRepository creates a new optimized widget repository
func NewOptimizedWidgetRepository(client *RedisClient, statsRepo StatsRepository) *OptimizedWidgetRepository {
	baseRepo := NewRedisWidgetRepository(client, statsRepo)
	return &OptimizedWidgetRepository{
		RedisWidgetRepository: baseRepo,
		typeCache:             NewWidgetTypeCache(client),
	}
}

// GetByUserIDWithFiltersOptimized provides optimized filtering with caching and batch operations
func (r *OptimizedWidgetRepository) GetByUserIDWithFiltersOptimized(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	// If no filters are applied, use the existing method for optimal performance
	if opts.Filters == nil || !opts.Filters.HasFilters() {
		return r.GetByUserID(ctx, userID, opts)
	}

	// Validate and clean filter options
	filters := models.ValidateFilterOptions(opts.Filters)
	if filters == nil || !filters.HasFilters() {
		return r.GetByUserID(ctx, userID, opts)
	}

	// Get filtered widget IDs using optimized Redis operations with caching
	filteredWidgetIDs, err := r.getFilteredWidgetIDsOptimized(ctx, userID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get filtered widget IDs: %w", err)
	}

	// Apply name search filter if specified (this can't be easily cached)
	if filters.HasSearchFilter() {
		filteredWidgetIDs, err = r.applyNameSearchFilterOptimized(ctx, filteredWidgetIDs, filters.Search)
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

	// Load widgets for the current page using optimized batch operations
	widgets, err := r.batchLoadWidgetsOptimized(ctx, paginatedWidgetIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to batch load widgets: %w", err)
	}

	return widgets, total, nil
}

// getFilteredWidgetIDsOptimized applies Redis-based filtering with caching optimizations
func (r *OptimizedWidgetRepository) getFilteredWidgetIDsOptimized(ctx context.Context, userID string, filters *models.FilterOptions) ([]string, error) {
	userWidgetsKey := GenerateUserWidgetsKey(userID)

	// If we only have user widgets (no additional filters), get all user widgets
	if !filters.HasTypeFilter() && !filters.HasVisibilityFilter() {
		// Get all user widgets sorted by creation time (newest first)
		widgetIDs, err := r.client.client.ZRevRange(ctx, userWidgetsKey, 0, -1).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get user widgets: %w", err)
		}
		return widgetIDs, nil
	}

	// For multiple type filters, we need special handling
	if filters.HasTypeFilter() && len(filters.Types) > 1 {
		return r.getFilteredWidgetIDsWithMultipleTypesOptimized(ctx, userID, filters)
	}

	// Build list of sets to intersect using cached data when possible
	setsToIntersect := []string{userWidgetsKey}

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

	// Pre-allocate interface slice for better performance
	userWidgetIDsInterface := make([]interface{}, len(userWidgetIDs))
	for i, id := range userWidgetIDs {
		userWidgetIDsInterface[i] = id
	}

	// Add user widgets to temporary SET
	if err := r.client.client.SAdd(ctx, tempUserSetKey, userWidgetIDsInterface...).Err(); err != nil {
		return nil, fmt.Errorf("failed to create temporary user set: %w", err)
	}

	// Replace userWidgetsKey with tempUserSetKey in the intersection
	setsToIntersect[0] = tempUserSetKey

	// Add type filter set
	if filters.HasTypeFilter() && len(filters.Types) == 1 {
		typeKey := GenerateWidgetsByTypeKey(filters.Types[0])
		setsToIntersect = append(setsToIntersect, typeKey)
	}

	// Add visibility filter set
	if filters.HasVisibilityFilter() {
		statusKey := GenerateWidgetsByStatusKey(*filters.IsVisible)
		setsToIntersect = append(setsToIntersect, statusKey)
	}

	// Single type or visibility filter - use direct SINTER
	widgetIDs, err := r.client.client.SInter(ctx, setsToIntersect...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to intersect widget sets: %w", err)
	}

	// Sort by creation time (newest first) by getting scores from user widgets zset
	return r.sortWidgetIDsByCreationTimeOptimized(ctx, userWidgetsKey, widgetIDs)
}

// getFilteredWidgetIDsWithMultipleTypesOptimized handles filtering when multiple types are specified with optimizations
func (r *OptimizedWidgetRepository) getFilteredWidgetIDsWithMultipleTypesOptimized(ctx context.Context, userID string, filters *models.FilterOptions) ([]string, error) {
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

	// Get all user widget IDs and add them to a temporary SET with pre-allocation
	userWidgetIDs, err := r.client.client.ZRange(ctx, userWidgetsKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user widgets: %w", err)
	}

	if len(userWidgetIDs) == 0 {
		return []string{}, nil
	}

	// Pre-allocate interface slice for better performance
	userWidgetIDsInterface := make([]interface{}, len(userWidgetIDs))
	for i, id := range userWidgetIDs {
		userWidgetIDsInterface[i] = id
	}

	// Add user widgets to temporary SET
	if err := r.client.client.SAdd(ctx, tempUserSetKey, userWidgetIDsInterface...).Err(); err != nil {
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
	return r.sortWidgetIDsByCreationTimeOptimized(ctx, userWidgetsKey, widgetIDs)
}

// sortWidgetIDsByCreationTimeOptimized sorts widget IDs with optimizations
func (r *OptimizedWidgetRepository) sortWidgetIDsByCreationTimeOptimized(ctx context.Context, userWidgetsKey string, widgetIDs []string) ([]string, error) {
	if len(widgetIDs) <= 1 {
		return widgetIDs, nil
	}

	// Use a more efficient sorting approach with pre-allocated structures
	// Pre-allocate slice with known capacity
	widgetsWithTime := make([]widgetWithTime, 0, len(widgetIDs))

	// Get creation times in batch using pipeline
	pipe := r.client.client.Pipeline()
	scoreCommands := make([]*redis.FloatCmd, len(widgetIDs))

	for i, widgetID := range widgetIDs {
		scoreCommands[i] = pipe.ZScore(ctx, userWidgetsKey, widgetID)
	}

	// Execute all commands at once
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get widget creation times: %w", err)
	}

	// Build slice with times
	for i, widgetID := range widgetIDs {
		if score, err := scoreCommands[i].Result(); err == nil {
			widgetsWithTime = append(widgetsWithTime, widgetWithTime{
				id:   widgetID,
				time: score,
			})
		}
	}

	// Sort by creation time (highest score/newest first) using standard sort
	sort.Slice(widgetsWithTime, func(i, j int) bool {
		return widgetsWithTime[i].time > widgetsWithTime[j].time // Descending order (newest first)
	})

	// Extract sorted widget IDs
	sortedWidgetIDs := make([]string, len(widgetsWithTime))
	for i, widget := range widgetsWithTime {
		sortedWidgetIDs[i] = widget.id
	}

	return sortedWidgetIDs, nil
}

// applyNameSearchFilterOptimized filters widget IDs by name search with optimizations
func (r *OptimizedWidgetRepository) applyNameSearchFilterOptimized(ctx context.Context, widgetIDs []string, searchTerm string) ([]string, error) {
	if len(widgetIDs) == 0 || searchTerm == "" {
		return widgetIDs, nil
	}

	// Trim and convert search term to lowercase for case-insensitive search
	searchLower := strings.ToLower(strings.TrimSpace(searchTerm))
	if searchLower == "" {
		return widgetIDs, nil
	}

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

	// Pre-allocate result slice with estimated capacity (50% match rate)
	estimatedMatches := len(widgetIDs) / 2
	if estimatedMatches < 10 {
		estimatedMatches = len(widgetIDs)
	}
	filteredIDs := make([]string, 0, estimatedMatches)

	// Filter based on names with optimized string operations
	for i, widgetID := range widgetIDs {
		name, err := nameCommands[i].Result()
		if err != nil {
			continue // Skip widgets that can't be loaded
		}

		// Case-insensitive name search with optimized string comparison
		if r.containsIgnoreCase(name, searchLower) {
			filteredIDs = append(filteredIDs, widgetID)
		}
	}

	return filteredIDs, nil
}

// containsIgnoreCase performs optimized case-insensitive string matching
func (r *OptimizedWidgetRepository) containsIgnoreCase(text, search string) bool {
	// Fast path for empty search
	if search == "" {
		return true
	}

	// Fast path for exact match
	if text == search {
		return true
	}

	// Convert to lowercase only once and check
	textLower := strings.ToLower(text)
	return strings.Contains(textLower, search)
}

// batchLoadWidgetsOptimized loads multiple widgets with optimizations
func (r *OptimizedWidgetRepository) batchLoadWidgetsOptimized(ctx context.Context, widgetIDs []string) ([]*models.Widget, error) {
	if len(widgetIDs) == 0 {
		return []*models.Widget{}, nil
	}

	// Pre-allocate result slice
	widgets := make([]*models.Widget, 0, len(widgetIDs))

	// Use larger pipeline batch size for better performance
	const batchSize = 50
	for i := 0; i < len(widgetIDs); i += batchSize {
		end := i + batchSize
		if end > len(widgetIDs) {
			end = len(widgetIDs)
		}

		batchWidgets, err := r.batchLoadWidgetsBatch(ctx, widgetIDs[i:end])
		if err != nil {
			return nil, err
		}

		widgets = append(widgets, batchWidgets...)
	}

	return widgets, nil
}

// batchLoadWidgetsBatch loads a batch of widgets efficiently
func (r *OptimizedWidgetRepository) batchLoadWidgetsBatch(ctx context.Context, widgetIDs []string) ([]*models.Widget, error) {
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

	// Pre-allocate result slice
	widgets := make([]*models.Widget, 0, len(widgetIDs))

	// Parse results with error handling
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

		// Parse stats data - this is optional
		if statsHash, err := statsCommands[i].Result(); err == nil && len(statsHash) > 0 {
			if stats := r.parseWidgetStats(widgetID, statsHash); stats != nil {
				widget.Stats = stats
			}
		}

		widgets = append(widgets, widget)
	}

	return widgets, nil
}

// parseWidgetStats parses widget stats from Redis hash with error handling
func (r *OptimizedWidgetRepository) parseWidgetStats(widgetID string, statsHash map[string]string) *models.WidgetStats {
	stats := &models.WidgetStats{WidgetID: widgetID}

	// Parse numeric fields with error handling
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

	return stats
}
