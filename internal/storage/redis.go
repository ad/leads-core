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
	client         redis.UniversalClient
	embeddedServer *EmbeddedRedisServer
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) {
	var client redis.UniversalClient
	var embeddedServer *EmbeddedRedisServer

	// Проверяем, нужно ли использовать встроенный сервер
	if cfg.UseEmbedded {
		// Создаем и запускаем встроенный Redis сервер
		var err error
		embeddedServer, err = NewEmbeddedRedisServer(cfg.EmbeddedPort, cfg.EmbeddedDBPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedded Redis server: %w", err)
		}

		if err := embeddedServer.Start(); err != nil {
			return nil, fmt.Errorf("failed to start embedded Redis server: %w", err)
		}

		// Создаем клиент для подключения к встроенному серверу
		client = redis.NewClient(&redis.Options{
			Addr: "localhost" + embeddedServer.GetAddr(),

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
	} else {
		// Используем внешний Redis
		// Check if we have cluster configuration by looking at multiple addresses
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
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		// Если встроенный сервер не удалось запустить, останавливаем его
		if embeddedServer != nil {
			embeddedServer.Stop()
		}
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client:         client,
		embeddedServer: embeddedServer,
	}, nil
}

// NewRedisClientWithUniversal creates a Redis client from UniversalClient
func NewRedisClientWithUniversal(client redis.UniversalClient) *RedisClient {
	return &RedisClient{client: client}
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	// Сначала закрываем клиент
	err := r.client.Close()

	// Затем останавливаем встроенный сервер, если он есть
	if r.embeddedServer != nil {
		if stopErr := r.embeddedServer.Stop(); stopErr != nil {
			if err == nil {
				err = stopErr
			}
		}
	}

	return err
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
	// Widgets - use {widgetID} hash tag to ensure related keys are in same slot
	WidgetKey          = "{%s}:widget"          // HASH - widget data
	WidgetsByTimeKey   = "widgets:by_time"      // ZSET - all widgets by timestamp (global)
	UserWidgetsKey     = "{%s}:user:widgets"    // SET - user's widgets
	WidgetsByTypeKey   = "widgets:type:%s"      // SET - widgets by type (global)
	WidgetsByStatusKey = "widgets:isVisible:%s" // SET - widgets by status (0|1) (global)

	// Submissions - use {widgetID} hash tag to group with widget data
	SubmissionKey        = "{%s}:submission:%s" // HASH - submission data
	WidgetSubmissionsKey = "{%s}:submissions"   // ZSET - widget submissions by timestamp

	// Statistics - use {widgetID} hash tag to group with widget data
	WidgetStatsKey = "{%s}:stats"    // HASH - widget statistics
	DailyViewsKey  = "{%s}:views:%s" // INCR - daily views (YYYY-MM-DD)

	// Rate limiting with hash tags for cluster compatibility
	RateLimitIPKey     = "rate_limit:{%s}:ip:%s"  // INCR - IP rate limit with hash tag
	RateLimitGlobalKey = "rate_limit:{%s}:global" // INCR - global rate limit with hash tag
)

// GenerateWidgetKey generates a widget key with hash tag
func GenerateWidgetKey(widgetID string) string {
	return fmt.Sprintf(WidgetKey, widgetID)
}

// GenerateUserWidgetsKey generates a user widgets key with hash tag
func GenerateUserWidgetsKey(userID string) string {
	return fmt.Sprintf(UserWidgetsKey, userID)
}

// GenerateWidgetsByTypeKey generates a widgets by type key
func GenerateWidgetsByTypeKey(widgetType string) string {
	return fmt.Sprintf(WidgetsByTypeKey, widgetType)
}

// GenerateWidgetsByStatusKey generates a widgets by status key
func GenerateWidgetsByStatusKey(enabled bool) string {
	status := "0"
	if enabled {
		status = "1"
	}
	return fmt.Sprintf(WidgetsByStatusKey, status)
}

// GenerateSubmissionKey generates a submission key with hash tag
func GenerateSubmissionKey(widgetID, submissionID string) string {
	return fmt.Sprintf(SubmissionKey, widgetID, submissionID)
}

// GenerateWidgetSubmissionsKey generates a widget submissions key with hash tag
func GenerateWidgetSubmissionsKey(widgetID string) string {
	return fmt.Sprintf(WidgetSubmissionsKey, widgetID)
}

// GenerateWidgetStatsKey generates a widget stats key with hash tag
func GenerateWidgetStatsKey(widgetID string) string {
	return fmt.Sprintf(WidgetStatsKey, widgetID)
}

// GenerateDailyViewsKey generates a daily views key with hash tag
func GenerateDailyViewsKey(widgetID, date string) string {
	return fmt.Sprintf(DailyViewsKey, widgetID, date)
}

// GenerateRateLimitIPKey generates a rate limit IP key
func GenerateRateLimitIPKey(ip, window string) string {
	return fmt.Sprintf(RateLimitIPKey, window, ip)
}

// GenerateRateLimitGlobalKey generates a rate limit global key
func GenerateRateLimitGlobalKey(window string) string {
	return fmt.Sprintf(RateLimitGlobalKey, window)
}
