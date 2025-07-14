package middleware

import (
	"net/http"
	"time"

	"github.com/ad/leads-core/pkg/logger"
)

// LoggingMiddleware logs HTTP requests with structured logging
type LoggingMiddleware struct {
	logger *logger.FieldLogger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger.WithFields(map[string]interface{}{
			"component": "http_logger",
		}),
	}
}

// LogRequests wraps an HTTP handler with request logging
func (lm *LoggingMiddleware) LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a wrapped response writer to capture status code and response size
		wrapped := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			responseSize:   0,
		}

		// Log request start
		lm.logger.Info("HTTP request started", map[string]interface{}{
			"method":      r.Method,
			"url":         r.URL.String(),
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
			"referer":     r.Referer(),
		})

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Determine log level based on status code
		logLevel := "info"
		if wrapped.statusCode >= 400 && wrapped.statusCode < 500 {
			logLevel = "warn"
		} else if wrapped.statusCode >= 500 {
			logLevel = "error"
		}

		// Log request completion
		fields := map[string]interface{}{
			"method":        r.Method,
			"url":           r.URL.String(),
			"status":        wrapped.statusCode,
			"duration_ms":   duration.Milliseconds(),
			"response_size": wrapped.responseSize,
			"remote_addr":   r.RemoteAddr,
		}

		switch logLevel {
		case "warn":
			lm.logger.Warn("HTTP request completed with client error", fields)
		case "error":
			lm.logger.Error("HTTP request completed with server error", fields)
		default:
			lm.logger.Info("HTTP request completed", fields)
		}

		// Log slow requests
		if duration > 1*time.Second {
			lm.logger.Warn("Slow HTTP request detected", map[string]interface{}{
				"method":      r.Method,
				"url":         r.URL.String(),
				"duration_ms": duration.Milliseconds(),
			})
		}
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture response details
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int
}

// WriteHeader captures the status code
func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.statusCode = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := lrw.ResponseWriter.Write(b)
	lrw.responseSize += size
	return size, err
}

// Global logging middleware instance
var defaultLoggingMiddleware *LoggingMiddleware

// InitLogging initializes the global logging middleware
func InitLogging() {
	defaultLoggingMiddleware = NewLoggingMiddleware()
}

// LogRequests provides access to the global logging middleware
func LogRequests(next http.Handler) http.Handler {
	if defaultLoggingMiddleware != nil {
		return defaultLoggingMiddleware.LogRequests(next)
	}
	return next
}
