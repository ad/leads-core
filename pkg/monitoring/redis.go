package monitoring

import (
	"context"
	"time"

	"github.com/ad/leads-core/pkg/logger"
	"github.com/ad/leads-core/pkg/metrics"
	"github.com/redis/go-redis/v9"
)

// RedisMonitor wraps Redis client with monitoring capabilities
type RedisMonitor struct {
	client redis.UniversalClient
	logger *logger.FieldLogger
}

// NewRedisMonitor creates a new Redis monitor wrapper
func NewRedisMonitor(client redis.UniversalClient) *RedisMonitor {
	return &RedisMonitor{
		client: client,
		logger: logger.WithFields(map[string]interface{}{
			"component": "redis_monitor",
		}),
	}
}

// WrapClient wraps Redis client methods with monitoring
func (rm *RedisMonitor) WrapClient() redis.UniversalClient {
	return &monitoredRedisClient{
		UniversalClient: rm.client,
		monitor:         rm,
	}
}

// monitoredRedisClient wraps Redis client with monitoring
type monitoredRedisClient struct {
	redis.UniversalClient
	monitor *RedisMonitor
}

// executeWithMonitoring wraps Redis operations with monitoring
func (rm *RedisMonitor) executeWithMonitoring(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()

	// Log operation start
	rm.logger.Debug("Redis operation started", map[string]interface{}{
		"operation": operation,
	})

	// Execute operation
	err := fn()
	duration := time.Since(start)

	// Record metrics
	labels := map[string]string{
		"operation": operation,
	}

	if err != nil {
		labels["status"] = "error"
		metrics.Inc("redis_operations_errors_total", labels, "Total Redis operation errors")

		rm.logger.Error("Redis operation failed", map[string]interface{}{
			"operation": operation,
			"error":     err.Error(),
			"duration":  duration.Milliseconds(),
		})
	} else {
		labels["status"] = "success"

		rm.logger.Debug("Redis operation completed", map[string]interface{}{
			"operation": operation,
			"duration":  duration.Milliseconds(),
		})
	}

	metrics.Inc("redis_operations_total", labels, "Total Redis operations")
	metrics.Observe("redis_operation_duration_seconds", duration.Seconds(), labels, "Redis operation duration in seconds")

	return err
}

// Override key Redis methods with monitoring
func (mrc *monitoredRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	var result *redis.StringCmd
	mrc.monitor.executeWithMonitoring(ctx, "GET", func() error {
		result = mrc.UniversalClient.Get(ctx, key)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	var result *redis.StatusCmd
	mrc.monitor.executeWithMonitoring(ctx, "SET", func() error {
		result = mrc.UniversalClient.Set(ctx, key, value, expiration)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	var result *redis.IntCmd
	mrc.monitor.executeWithMonitoring(ctx, "HSET", func() error {
		result = mrc.UniversalClient.HSet(ctx, key, values...)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) HMSet(ctx context.Context, key string, values ...interface{}) *redis.BoolCmd {
	var result *redis.BoolCmd
	mrc.monitor.executeWithMonitoring(ctx, "HMSET", func() error {
		result = mrc.UniversalClient.HMSet(ctx, key, values...)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	var result *redis.MapStringStringCmd
	mrc.monitor.executeWithMonitoring(ctx, "HGETALL", func() error {
		result = mrc.UniversalClient.HGetAll(ctx, key)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	var result *redis.IntCmd
	mrc.monitor.executeWithMonitoring(ctx, "DEL", func() error {
		result = mrc.UniversalClient.Del(ctx, keys...)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	var result *redis.IntCmd
	mrc.monitor.executeWithMonitoring(ctx, "ZADD", func() error {
		result = mrc.UniversalClient.ZAdd(ctx, key, members...)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) ZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	var result *redis.StringSliceCmd
	mrc.monitor.executeWithMonitoring(ctx, "ZRANGE", func() error {
		result = mrc.UniversalClient.ZRange(ctx, key, start, stop)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) ZRevRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	var result *redis.StringSliceCmd
	mrc.monitor.executeWithMonitoring(ctx, "ZREVRANGE", func() error {
		result = mrc.UniversalClient.ZRevRange(ctx, key, start, stop)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	var result *redis.IntCmd
	mrc.monitor.executeWithMonitoring(ctx, "SADD", func() error {
		result = mrc.UniversalClient.SAdd(ctx, key, members...)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	var result *redis.StringSliceCmd
	mrc.monitor.executeWithMonitoring(ctx, "SMEMBERS", func() error {
		result = mrc.UniversalClient.SMembers(ctx, key)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	var result *redis.IntCmd
	mrc.monitor.executeWithMonitoring(ctx, "INCR", func() error {
		result = mrc.UniversalClient.Incr(ctx, key)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	var result *redis.BoolCmd
	mrc.monitor.executeWithMonitoring(ctx, "EXPIRE", func() error {
		result = mrc.UniversalClient.Expire(ctx, key, expiration)
		return result.Err()
	})
	return result
}

func (mrc *monitoredRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	var result *redis.StatusCmd
	mrc.monitor.executeWithMonitoring(ctx, "PING", func() error {
		result = mrc.UniversalClient.Ping(ctx)
		return result.Err()
	})
	return result
}

// ConnectionMonitor monitors Redis connection health
type ConnectionMonitor struct {
	client redis.UniversalClient
	logger *logger.FieldLogger
}

// NewConnectionMonitor creates a new connection monitor
func NewConnectionMonitor(client redis.UniversalClient) *ConnectionMonitor {
	return &ConnectionMonitor{
		client: client,
		logger: logger.WithFields(map[string]interface{}{
			"component": "redis_connection_monitor",
		}),
	}
}

// MonitorHealth checks Redis connection health and records metrics
func (cm *ConnectionMonitor) MonitorHealth(ctx context.Context) {
	start := time.Now()

	err := cm.client.Ping(ctx).Err()
	duration := time.Since(start)

	if err != nil {
		metrics.Inc("redis_connection_errors_total", nil, "Total Redis connection errors")
		metrics.Set("redis_connection_up", 0, nil, "Redis connection status (1=up, 0=down)")

		cm.logger.Error("Redis connection health check failed", map[string]interface{}{
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		})
	} else {
		metrics.Set("redis_connection_up", 1, nil, "Redis connection status (1=up, 0=down)")

		cm.logger.Debug("Redis connection health check successful", map[string]interface{}{
			"duration": duration.Milliseconds(),
		})
	}

	metrics.Observe("redis_ping_duration_seconds", duration.Seconds(), nil, "Redis ping duration in seconds")
}

// StartHealthCheck starts periodic health checks
func (cm *ConnectionMonitor) StartHealthCheck(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cm.logger.Info("Starting Redis health check monitor", map[string]interface{}{
		"interval": interval.String(),
	})

	// Initial health check
	cm.MonitorHealth(ctx)

	for {
		select {
		case <-ctx.Done():
			cm.logger.Info("Redis health check monitor stopped")
			return
		case <-ticker.C:
			cm.MonitorHealth(ctx)
		}
	}
}
