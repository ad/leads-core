package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/ad/leads-core/internal/models"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	startTime time.Time
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		startTime: time.Now(),
	}
}

// Metrics handles GET /metrics
func (h *MetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := models.MetricsResponse{
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startTime),
		Memory: models.MemoryMetrics{
			Alloc:      memStats.Alloc,
			TotalAlloc: memStats.TotalAlloc,
			Sys:        memStats.Sys,
			HeapAlloc:  memStats.HeapAlloc,
			HeapSys:    memStats.HeapSys,
		},
		Runtime: models.RuntimeMetrics{
			Goroutines: runtime.NumGoroutine(),
			GCRuns:     memStats.NumGC,
			NextGC:     memStats.NextGC,
			LastGC:     time.Unix(0, int64(memStats.LastGC)),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Data: metrics})
}
