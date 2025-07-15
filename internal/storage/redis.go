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
	var client redis.UniversalClient

	// Check if we have cluster configuration by looking at multiple addresses
	// or specific cluster indicators in the first address
	isCluster := len(cfg.Addresses) > 1

	// // Also check if the first address contains a cluster port pattern (7000-7999)
	// if len(cfg.Addresses) == 1 {
	// 	addr := cfg.Addresses[0]
	// 	// Check for cluster port pattern
	// 	if strings.Contains(addr, ":700") || strings.Contains(addr, ":701") ||
	// 		strings.Contains(addr, ":702") || strings.Contains(addr, ":703") ||
	// 		strings.Contains(addr, ":704") || strings.Contains(addr, ":705") ||
	// 		strings.Contains(addr, ":706") || strings.Contains(addr, ":707") ||
	// 		strings.Contains(addr, ":708") || strings.Contains(addr, ":709") {
	// 		isCluster = true
	// 	}
	// }

	if isCluster {
		// Use cluster client
		client = redis.NewClusterClient(&redis.ClusterOptions{
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
	} else {
		// Single Redis instance
		client = redis.NewClient(&redis.Options{
			Addr:     cfg.Addresses[0],
			Password: cfg.Password,
			DB:       cfg.DB,

			// Connection pool optimization
			PoolSize:        50,
			PoolTimeout:     30 * time.Second,
			MaxRetries:      3,
			MinRetryBackoff: 8 * time.Millisecond,
			MaxRetryBackoff: 512 * time.Millisecond,

			// Timeouts
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		})
	}

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

// Redis key patterns with hash tags for cluster compatibility
const (
	// Forms - use {formID} hash tag to ensure related keys are in same slot
	FormKey          = "{%s}:form"        // HASH - form data
	FormsByTimeKey   = "forms:by_time"    // ZSET - all forms by timestamp (global)
	UserFormsKey     = "{%s}:user:forms"  // SET - user's forms
	FormsByTypeKey   = "forms:type:%s"    // SET - forms by type (global)
	FormsByStatusKey = "forms:enabled:%s" // SET - forms by status (0|1) (global)

	// Submissions - use {formID} hash tag to group with form data
	SubmissionKey      = "{%s}:submission:%s" // HASH - submission data
	FormSubmissionsKey = "{%s}:submissions"   // ZSET - form submissions by timestamp

	// Statistics - use {formID} hash tag to group with form data
	FormStatsKey  = "{%s}:stats"    // HASH - form statistics
	DailyViewsKey = "{%s}:views:%s" // INCR - daily views (YYYY-MM-DD)

	// Rate limiting with hash tags for cluster compatibility
	RateLimitIPKey     = "rate_limit:{%s}:ip:%s"  // INCR - IP rate limit with hash tag
	RateLimitGlobalKey = "rate_limit:{%s}:global" // INCR - global rate limit with hash tag
)

// GenerateFormKey generates a form key with hash tag
func GenerateFormKey(formID string) string {
	return fmt.Sprintf(FormKey, formID)
}

// GenerateUserFormsKey generates a user forms key with hash tag
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

// GenerateSubmissionKey generates a submission key with hash tag
func GenerateSubmissionKey(formID, submissionID string) string {
	return fmt.Sprintf(SubmissionKey, formID, submissionID)
}

// GenerateFormSubmissionsKey generates a form submissions key with hash tag
func GenerateFormSubmissionsKey(formID string) string {
	return fmt.Sprintf(FormSubmissionsKey, formID)
}

// GenerateFormStatsKey generates a form stats key with hash tag
func GenerateFormStatsKey(formID string) string {
	return fmt.Sprintf(FormStatsKey, formID)
}

// GenerateDailyViewsKey generates a daily views key with hash tag
func GenerateDailyViewsKey(formID, date string) string {
	return fmt.Sprintf(DailyViewsKey, formID, date)
}

// GenerateRateLimitIPKey generates a rate limit IP key
func GenerateRateLimitIPKey(ip, window string) string {
	return fmt.Sprintf(RateLimitIPKey, window, ip)
}

// GenerateRateLimitGlobalKey generates a rate limit global key
func GenerateRateLimitGlobalKey(window string) string {
	return fmt.Sprintf(RateLimitGlobalKey, window)
}
