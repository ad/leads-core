package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ad/leads-core/internal/config"
	"github.com/ad/leads-core/internal/storage"
	"github.com/ad/leads-core/pkg/logger"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	client *storage.RedisClient
	config config.RateLimitConfig
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *storage.RedisClient, config config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		client: client,
		config: config,
	}
}

// RateLimit middleware for rate limiting requests
func (rl *RateLimiter) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract IP address
		ip := getClientIP(r)
		if ip == "" {
			logger.Error("Failed to extract client IP for rate limiting", map[string]interface{}{
				"action": "rate_limit",
				"error":  "failed to extract client IP",
			})
			writeErrorResponse(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		// Check rate limits
		if exceeded, err := rl.checkRateLimit(ctx, ip); err != nil {
			logger.Error("Rate limit check failed", map[string]interface{}{
				"action": "rate_limit",
				"ip":     ip,
				"error":  err.Error(),
			})
			writeErrorResponse(w, http.StatusInternalServerError, "Internal server error")
			return
		} else if exceeded {
			logger.Warn("Rate limit exceeded", map[string]interface{}{
				"action": "rate_limit",
				"ip":     ip,
				"status": "exceeded",
			})
			writeErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// checkRateLimit checks both IP and global rate limits
func (rl *RateLimiter) checkRateLimit(ctx context.Context, ip string) (bool, error) {
	now := time.Now()
	window := now.Format("2006-01-02T15:04") // 1-minute window

	pipe := rl.client.GetClient().TxPipeline()

	// Check IP rate limit
	ipKey := storage.GenerateRateLimitIPKey(ip, window)
	ipCountCmd := pipe.Incr(ctx, ipKey)
	pipe.Expire(ctx, ipKey, time.Minute)

	// Check global rate limit
	globalKey := storage.GenerateRateLimitGlobalKey(window)
	globalCountCmd := pipe.Incr(ctx, globalKey)
	pipe.Expire(ctx, globalKey, time.Minute)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	// Check limits
	ipCount := ipCountCmd.Val()
	globalCount := globalCountCmd.Val()

	if ipCount > int64(rl.config.IPPerMinute) {
		return true, nil
	}

	if globalCount > int64(rl.config.GlobalPerMinute) {
		return true, nil
	}

	return false, nil
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP from the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	if net.ParseIP(host) != nil {
		return host
	}

	return ""
}
