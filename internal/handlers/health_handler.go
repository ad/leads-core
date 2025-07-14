package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/storage"
)

// HealthHandler handles system health endpoints
type HealthHandler struct {
	redisClient *storage.RedisClient
	startTime   time.Time
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    string            `json:"uptime"`
	Services  map[string]string `json:"services"`
	Metrics   HealthMetrics     `json:"metrics"`
}

// HealthMetrics represents system metrics
type HealthMetrics struct {
	MemoryUsage float64 `json:"memory_usage_mb"`
	CPUUsage    float64 `json:"cpu_usage_percent"`
	Goroutines  int     `json:"goroutines"`
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(redisClient *storage.RedisClient) *HealthHandler {
	return &HealthHandler{
		redisClient: redisClient,
		startTime:   time.Now(),
	}
}

// Health handles GET /health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := "ok"
	services := make(map[string]string)

	// Check Redis connection
	if err := h.redisClient.Ping(r.Context()); err != nil {
		status = "error"
		services["redis"] = "error: " + err.Error()
	} else {
		services["redis"] = "ok"
	}

	// Get system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := HealthMetrics{
		MemoryUsage: float64(m.Alloc) / 1024 / 1024, // Convert to MB
		CPUUsage:    0.0,                            // Would need additional implementation for CPU usage
		Goroutines:  runtime.NumGoroutine(),
	}

	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startTime).String(),
		Services:  services,
		Metrics:   metrics,
	}

	statusCode := http.StatusOK
	if status == "error" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSONResponse(w, statusCode, models.Response{Data: response})
}
