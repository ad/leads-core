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
	IncrementViews(ctx context.Context, formID string) error
	IncrementSubmits(ctx context.Context, formID string) error
	IncrementCloses(ctx context.Context, formID string) error
	GetFormStats(ctx context.Context, formID string) (*models.FormStats, error)
	GetDailyViews(ctx context.Context, formID, date string) (int64, error)
}

// RedisStatsRepository implements StatsRepository for Redis
type RedisStatsRepository struct {
	client *RedisClient
}

// NewRedisStatsRepository creates a new Redis stats repository
func NewRedisStatsRepository(client *RedisClient) *RedisStatsRepository {
	return &RedisStatsRepository{client: client}
}

// IncrementViews increments view count for a form
func (r *RedisStatsRepository) IncrementViews(ctx context.Context, formID string) error {
	pipe := r.client.client.TxPipeline()

	// Increment total views
	statsKey := GenerateFormStatsKey(formID)
	pipe.HIncrBy(ctx, statsKey, "views", 1)
	pipe.HSet(ctx, statsKey, "last_view", time.Now().Unix())

	// Increment daily views
	today := time.Now().Format("2006-01-02")
	dailyKey := GenerateDailyViewsKey(formID, today)
	pipe.Incr(ctx, dailyKey)
	pipe.Expire(ctx, dailyKey, 30*24*time.Hour) // Keep daily stats for 30 days

	_, err := pipe.Exec(ctx)
	return err
}

// IncrementSubmits increments submit count for a form
func (r *RedisStatsRepository) IncrementSubmits(ctx context.Context, formID string) error {
	statsKey := GenerateFormStatsKey(formID)
	return r.client.client.HIncrBy(ctx, statsKey, "submits", 1).Err()
}

// IncrementCloses increments close count for a form
func (r *RedisStatsRepository) IncrementCloses(ctx context.Context, formID string) error {
	statsKey := GenerateFormStatsKey(formID)
	return r.client.client.HIncrBy(ctx, statsKey, "closes", 1).Err()
}

// GetFormStats retrieves statistics for a form
func (r *RedisStatsRepository) GetFormStats(ctx context.Context, formID string) (*models.FormStats, error) {
	statsKey := GenerateFormStatsKey(formID)
	hash, err := r.client.client.HGetAll(ctx, statsKey).Result()
	if err != nil {
		return nil, err
	}

	if len(hash) == 0 {
		// Return empty stats if not found
		return &models.FormStats{
			FormID:  formID,
			Views:   0,
			Submits: 0,
			Closes:  0,
		}, nil
	}

	stats := &models.FormStats{FormID: formID}

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
func (r *RedisStatsRepository) GetDailyViews(ctx context.Context, formID, date string) (int64, error) {
	dailyKey := GenerateDailyViewsKey(formID, date)
	count, err := r.client.client.Get(ctx, dailyKey).Int64()
	if err == redis.Nil {
		return 0, nil // No views for this date
	}
	return count, err
}
