package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/redis/go-redis/v9"
)

// StatsRepository defines interface for statistics operations
type StatsRepository interface {
	IncrementViews(ctx context.Context, widgetID string) error
	IncrementSubmits(ctx context.Context, widgetID string) error
	IncrementCloses(ctx context.Context, widgetID string) error
	GetWidgetStats(ctx context.Context, widgetID string) (*models.WidgetStats, error)
	GetDailyViews(ctx context.Context, widgetID, date string) (int64, error)
}

// RedisStatsRepository implements StatsRepository for Redis
type RedisStatsRepository struct {
	client *RedisClient
}

// NewRedisStatsRepository creates a new Redis stats repository
func NewRedisStatsRepository(client *RedisClient) *RedisStatsRepository {
	return &RedisStatsRepository{client: client}
}

// IncrementViews increments view count for a widget
func (r *RedisStatsRepository) IncrementViews(ctx context.Context, widgetID string) error {
	// All keys use {widgetID} hash tag, so they'll be in same slot
	pipe := r.client.client.TxPipeline()

	// Increment total views
	statsKey := GenerateWidgetStatsKey(widgetID)
	pipe.HIncrBy(ctx, statsKey, "views", 1)
	pipe.HSet(ctx, statsKey, "last_view", time.Now().Unix())

	// Increment daily views (same slot due to hash tag)
	today := time.Now().Format("2006-01-02")
	dailyKey := GenerateDailyViewsKey(widgetID, today)
	pipe.Incr(ctx, dailyKey)
	pipe.Expire(ctx, dailyKey, 30*24*time.Hour) // Keep daily stats for 30 days

	_, err := pipe.Exec(ctx)
	return err
}

// IncrementSubmits increments submit count for a widget
func (r *RedisStatsRepository) IncrementSubmits(ctx context.Context, widgetID string) error {
	statsKey := GenerateWidgetStatsKey(widgetID)
	return r.client.client.HIncrBy(ctx, statsKey, "submits", 1).Err()
}

// IncrementCloses increments close count for a widget
func (r *RedisStatsRepository) IncrementCloses(ctx context.Context, widgetID string) error {
	statsKey := GenerateWidgetStatsKey(widgetID)
	return r.client.client.HIncrBy(ctx, statsKey, "closes", 1).Err()
}

// GetWidgetStats retrieves statistics for a widget
func (r *RedisStatsRepository) GetWidgetStats(ctx context.Context, widgetID string) (*models.WidgetStats, error) {
	statsKey := GenerateWidgetStatsKey(widgetID)
	hash, err := r.client.client.HGetAll(ctx, statsKey).Result()
	if err != nil {
		return nil, err
	}

	if len(hash) == 0 {
		// Return empty stats if not found
		return &models.WidgetStats{
			WidgetID: widgetID,
			Views:    0,
			Submits:  0,
			Closes:   0,
		}, nil
	}

	stats := &models.WidgetStats{WidgetID: widgetID}

	if viewsStr, ok := hash["views"]; ok {
		if views, err := strconv.ParseInt(viewsStr, 10, 64); err == nil {
			stats.Views = views
		}
	}

	if submitsStr, ok := hash["submits"]; ok {
		if submits, err := strconv.ParseInt(submitsStr, 10, 64); err == nil {
			stats.Submits = submits
		}
	}

	if closesStr, ok := hash["closes"]; ok {
		if closes, err := strconv.ParseInt(closesStr, 10, 64); err == nil {
			stats.Closes = closes
		}
	}

	if lastViewStr, ok := hash["last_view"]; ok {
		if timestamp, err := strconv.ParseInt(lastViewStr, 10, 64); err == nil {
			stats.LastView = time.Unix(timestamp, 0)
		}
	}

	return stats, nil
}

// GetDailyViews retrieves daily view count for a specific date
func (r *RedisStatsRepository) GetDailyViews(ctx context.Context, widgetID, date string) (int64, error) {
	dailyKey := GenerateDailyViewsKey(widgetID, date)
	count, err := r.client.client.Get(ctx, dailyKey).Int64()
	if err == redis.Nil {
		return 0, nil // No views for this date
	}
	return count, err
}
