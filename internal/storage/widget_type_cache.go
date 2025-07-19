package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ad/leads-core/internal/models"
)

// WidgetTypeCache provides caching for widget type metadata
// This helps optimize filtering operations by reducing Redis calls
type WidgetTypeCache struct {
	client          *RedisClient
	cache           map[string][]string // type -> widget IDs
	visibilityCache map[bool][]string   // visibility -> widget IDs
	mutex           sync.RWMutex
	lastUpdate      time.Time
	ttl             time.Duration
}

// NewWidgetTypeCache creates a new widget type cache
func NewWidgetTypeCache(client *RedisClient) *WidgetTypeCache {
	return &WidgetTypeCache{
		client:          client,
		cache:           make(map[string][]string),
		visibilityCache: make(map[bool][]string),
		ttl:             5 * time.Minute, // Cache for 5 minutes
	}
}

// GetWidgetIDsByType returns widget IDs for a specific type with caching
func (c *WidgetTypeCache) GetWidgetIDsByType(ctx context.Context, widgetType string) ([]string, error) {
	c.mutex.RLock()
	if time.Since(c.lastUpdate) < c.ttl {
		if ids, ok := c.cache[widgetType]; ok {
			c.mutex.RUnlock()
			return ids, nil
		}
	}
	c.mutex.RUnlock()

	// Cache miss or expired, fetch from Redis
	return c.refreshTypeCache(ctx, widgetType)
}

// GetWidgetIDsByVisibility returns widget IDs for a specific visibility state with caching
func (c *WidgetTypeCache) GetWidgetIDsByVisibility(ctx context.Context, isVisible bool) ([]string, error) {
	c.mutex.RLock()
	if time.Since(c.lastUpdate) < c.ttl {
		if ids, ok := c.visibilityCache[isVisible]; ok {
			c.mutex.RUnlock()
			return ids, nil
		}
	}
	c.mutex.RUnlock()

	// Cache miss or expired, fetch from Redis
	return c.refreshVisibilityCache(ctx, isVisible)
}

// refreshTypeCache refreshes the cache for a specific widget type
func (c *WidgetTypeCache) refreshTypeCache(ctx context.Context, widgetType string) ([]string, error) {
	typeKey := GenerateWidgetsByTypeKey(widgetType)
	ids, err := c.client.client.SMembers(ctx, typeKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get widget IDs by type %s: %w", widgetType, err)
	}

	c.mutex.Lock()
	c.cache[widgetType] = ids
	c.lastUpdate = time.Now()
	c.mutex.Unlock()

	return ids, nil
}

// refreshVisibilityCache refreshes the cache for a specific visibility state
func (c *WidgetTypeCache) refreshVisibilityCache(ctx context.Context, isVisible bool) ([]string, error) {
	statusKey := GenerateWidgetsByStatusKey(isVisible)
	ids, err := c.client.client.SMembers(ctx, statusKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get widget IDs by visibility %t: %w", isVisible, err)
	}

	c.mutex.Lock()
	c.visibilityCache[isVisible] = ids
	c.lastUpdate = time.Now()
	c.mutex.Unlock()

	return ids, nil
}

// InvalidateCache clears all cached data
func (c *WidgetTypeCache) InvalidateCache() {
	c.mutex.Lock()
	c.cache = make(map[string][]string)
	c.visibilityCache = make(map[bool][]string)
	c.lastUpdate = time.Time{}
	c.mutex.Unlock()
}

// WarmUp pre-loads cache with commonly used widget types
func (c *WidgetTypeCache) WarmUp(ctx context.Context) error {
	commonTypes := models.AllWidgetTypes()

	for _, widgetType := range commonTypes {
		if _, err := c.refreshTypeCache(ctx, widgetType); err != nil {
			// Log error but don't fail the warm-up
			continue
		}
	}

	// Also warm up visibility caches
	c.refreshVisibilityCache(ctx, true)
	c.refreshVisibilityCache(ctx, false)

	return nil
}
