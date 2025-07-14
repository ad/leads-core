package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/internal/config"
	"github.com/redis/go-redis/v9"
)

// RedisClient wraps Redis cluster client
type RedisClient struct {
	client redis.UniversalClient
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    cfg.Addresses,
		Password: cfg.Password,

		// Connection pool optimization
		PoolSize:        50, // Maximum number of connections per shard
		PoolTimeout:     30 * time.Second,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,

		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// NewRedisClientWithUniversal creates a Redis client from UniversalClient
func NewRedisClientWithUniversal(client redis.UniversalClient) *RedisClient {
	return &RedisClient{client: client}
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Ping checks Redis connection
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// GetClient returns the underlying Redis client
func (r *RedisClient) GetClient() redis.UniversalClient {
	return r.client
}

// Redis key patterns
const (
	// Forms
	FormKey          = "form:%s"          // HASH - form data
	FormsByTimeKey   = "forms:by_time"    // ZSET - all forms by timestamp
	UserFormsKey     = "forms:%s"         // SET - user's forms
	FormsByTypeKey   = "forms:type:%s"    // SET - forms by type
	FormsByStatusKey = "forms:enabled:%s" // SET - forms by status (0|1)

	// Submissions
	SubmissionKey      = "submission:%s:%s"    // HASH - submission data
	FormSubmissionsKey = "form:%s:submissions" // ZSET - form submissions by timestamp

	// Statistics
	FormStatsKey  = "form:%s:stats"          // HASH - form statistics
	DailyViewsKey = "stats:form:%s:views:%s" // INCR - daily views (YYYY-MM-DD)

	// Rate limiting
	RateLimitIPKey     = "rate_limit:ip:%s:%s"  // INCR - IP rate limit
	RateLimitGlobalKey = "rate_limit:global:%s" // INCR - global rate limit
)

// GenerateFormKey generates a form key
func GenerateFormKey(formID string) string {
	return fmt.Sprintf(FormKey, formID)
}

// GenerateUserFormsKey generates a user forms key
func GenerateUserFormsKey(userID string) string {
	return fmt.Sprintf(UserFormsKey, userID)
}

// GenerateFormsByTypeKey generates a forms by type key
func GenerateFormsByTypeKey(formType string) string {
	return fmt.Sprintf(FormsByTypeKey, formType)
}

// GenerateFormsByStatusKey generates a forms by status key
func GenerateFormsByStatusKey(enabled bool) string {
	status := "0"
	if enabled {
		status = "1"
	}
	return fmt.Sprintf(FormsByStatusKey, status)
}

// GenerateSubmissionKey generates a submission key
func GenerateSubmissionKey(formID, submissionID string) string {
	return fmt.Sprintf(SubmissionKey, formID, submissionID)
}

// GenerateFormSubmissionsKey generates a form submissions key
func GenerateFormSubmissionsKey(formID string) string {
	return fmt.Sprintf(FormSubmissionsKey, formID)
}

// GenerateFormStatsKey generates a form stats key
func GenerateFormStatsKey(formID string) string {
	return fmt.Sprintf(FormStatsKey, formID)
}

// GenerateDailyViewsKey generates a daily views key
func GenerateDailyViewsKey(formID, date string) string {
	return fmt.Sprintf(DailyViewsKey, formID, date)
}

// GenerateRateLimitIPKey generates a rate limit IP key
func GenerateRateLimitIPKey(ip, window string) string {
	return fmt.Sprintf(RateLimitIPKey, ip, window)
}

// GenerateRateLimitGlobalKey generates a rate limit global key
func GenerateRateLimitGlobalKey(window string) string {
	return fmt.Sprintf(RateLimitGlobalKey, window)
}
