package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// MockRedisClient for rate limiting tests
type MockRedisClientForRL struct {
	client redis.UniversalClient
}

func setupTestRedisForRL(t *testing.T) *MockRedisClientForRL {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	t.Cleanup(func() {
		mr.Close()
	})

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return &MockRedisClientForRL{client: client}
}

// TestRateLimiter is a simplified version for testing
type TestRateLimiter struct {
	client *MockRedisClientForRL
	config config.RateLimitConfig
}

func (rl *TestRateLimiter) CheckRateLimit(ip string) (bool, error) {
	ctx := context.Background()
	window := time.Now().Unix() / 60 // 1-minute windows

	// Create rate limit key
	key := "rate_limit:ip:" + ip + ":" + string(rune(window))

	// Increment counter
	count, err := rl.client.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// Set expiration on first request
	if count == 1 {
		rl.client.client.Expire(ctx, key, time.Minute)
	}

	return int(count) > rl.config.IPPerMinute, nil
}

func TestRateLimiter_CheckRateLimit(t *testing.T) {
	redis := setupTestRedisForRL(t)

	config := config.RateLimitConfig{
		IPPerMinute:     2,
		GlobalPerMinute: 10,
	}

	limiter := &TestRateLimiter{
		client: redis,
		config: config,
	}

	tests := []struct {
		name           string
		ip             string
		requests       int
		expectExceeded bool
	}{
		{
			name:           "within limit",
			ip:             "192.168.1.1",
			requests:       1,
			expectExceeded: false,
		},
		{
			name:           "at limit",
			ip:             "192.168.1.2",
			requests:       2,
			expectExceeded: false,
		},
		{
			name:           "exceed limit",
			ip:             "192.168.1.3",
			requests:       3,
			expectExceeded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make requests up to the limit
			for i := 0; i < tt.requests-1; i++ {
				exceeded, err := limiter.CheckRateLimit(tt.ip)
				if err != nil {
					t.Fatalf("Unexpected error on request %d: %v", i+1, err)
				}
				if exceeded {
					t.Fatalf("Rate limit exceeded unexpectedly on request %d", i+1)
				}
			}

			// Make the final request
			exceeded, err := limiter.CheckRateLimit(tt.ip)
			if err != nil {
				t.Fatalf("Unexpected error on final request: %v", err)
			}

			if exceeded != tt.expectExceeded {
				t.Errorf("Expected exceeded=%t, got %t", tt.expectExceeded, exceeded)
			}
		})
	}
}

func TestRateLimitMiddleware_Integration(t *testing.T) {
	tests := []struct {
		name           string
		requests       int
		expectedStatus []int
	}{
		{
			name:           "first request should succeed",
			requests:       1,
			expectedStatus: []int{http.StatusOK},
		},
		{
			name:           "second request should succeed",
			requests:       2,
			expectedStatus: []int{http.StatusOK, http.StatusOK},
		},
		{
			name:           "third request should fail",
			requests:       3,
			expectedStatus: []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh Redis instance for each test
			redis := setupTestRedisForRL(t)

			// Create a simplified middleware test
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			config := config.RateLimitConfig{
				IPPerMinute:     2,
				GlobalPerMinute: 10,
			}

			limiter := &TestRateLimiter{
				client: redis,
				config: config,
			}

			// Middleware wrapper
			middleware := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ip := getTestIP(r)
					exceeded, err := limiter.CheckRateLimit(ip)
					if err != nil {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
					if exceeded {
						http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
						return
					}
					next.ServeHTTP(w, r)
				})
			}

			wrappedHandler := middleware(handler)

			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.100:1234" // Same IP for all requests

				w := httptest.NewRecorder()
				wrappedHandler.ServeHTTP(w, req)

				expectedStatus := tt.expectedStatus[i]
				if w.Code != expectedStatus {
					t.Errorf("Request %d: expected status %d, got %d", i+1, expectedStatus, w.Code)
				}
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expectedIP string
	}{
		{
			name:       "from RemoteAddr",
			remoteAddr: "192.168.1.1:8080",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "from X-Forwarded-For header",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 198.51.100.1",
			},
			expectedIP: "203.0.113.1",
		},
		{
			name:       "from X-Real-IP header",
			remoteAddr: "10.0.0.1:8080",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			expectedIP: "203.0.113.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			ip := getTestIP(req)
			if ip != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// getTestIP simplified version of getClientIP for testing
func getTestIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP from the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}

func TestRateLimitDifferentIPs(t *testing.T) {
	redis := setupTestRedisForRL(t)

	config := config.RateLimitConfig{
		IPPerMinute:     1,
		GlobalPerMinute: 10,
	}

	limiter := &TestRateLimiter{
		client: redis,
		config: config,
	}

	// Different IPs should have separate rate limits
	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// First IP: first request should pass
	exceeded, err := limiter.CheckRateLimit(ip1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if exceeded {
		t.Error("First request for IP1 should not be rate limited")
	}

	// Second IP: first request should also pass
	exceeded, err = limiter.CheckRateLimit(ip2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if exceeded {
		t.Error("First request for IP2 should not be rate limited")
	}

	// First IP: second request should be limited
	exceeded, err = limiter.CheckRateLimit(ip1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !exceeded {
		t.Error("Second request for IP1 should be rate limited")
	}

	// Second IP: second request should be limited
	exceeded, err = limiter.CheckRateLimit(ip2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !exceeded {
		t.Error("Second request for IP2 should be rate limited")
	}
}
