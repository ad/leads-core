package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	_ "net/http/pprof" // Import pprof for profiling endpoints
	"runtime"
	"time"

	"github.com/ad/leads-core/pkg/logger"
	"github.com/ad/leads-core/pkg/metrics"
)

// ProfileMonitor provides application profiling capabilities
type ProfileMonitor struct {
	logger *logger.FieldLogger
}

// NewProfileMonitor creates a new profile monitor
func NewProfileMonitor() *ProfileMonitor {
	return &ProfileMonitor{
		logger: logger.WithFields(map[string]interface{}{
			"component": "profile_monitor",
		}),
	}
}

// StartProfilingEndpoints starts HTTP endpoints for profiling
func (pm *ProfileMonitor) StartProfilingEndpoints(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	// pprof endpoints are automatically registered
	mux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/cmdline", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/profile", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/symbol", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/pprof/trace", http.DefaultServeMux.ServeHTTP)

	// Custom application metrics endpoint
	mux.HandleFunc("/debug/metrics", pm.metricsHandler)
	mux.HandleFunc("/debug/health", pm.healthHandler)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	pm.logger.Info("Starting profiling server", map[string]interface{}{
		"addr": addr,
	})

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}

// metricsHandler provides custom metrics endpoint
func (pm *ProfileMonitor) metricsHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Record current memory metrics
	metrics.Set("runtime_memory_alloc_bytes", float64(m.Alloc), nil, "Allocated memory")
	metrics.Set("runtime_memory_total_alloc_bytes", float64(m.TotalAlloc), nil, "Total allocated memory")
	metrics.Set("runtime_memory_sys_bytes", float64(m.Sys), nil, "System memory")
	metrics.Set("runtime_memory_heap_alloc_bytes", float64(m.HeapAlloc), nil, "Heap allocated memory")
	metrics.Set("runtime_memory_heap_sys_bytes", float64(m.HeapSys), nil, "Heap system memory")
	metrics.Set("runtime_memory_heap_objects", float64(m.HeapObjects), nil, "Heap objects")
	metrics.Set("runtime_goroutines", float64(runtime.NumGoroutine()), nil, "Number of goroutines")
	metrics.Set("runtime_gc_runs", float64(m.NumGC), nil, "Number of GC runs")

	// Use the global metrics handler
	metrics.Handler()(w, r)
}

// healthHandler provides detailed health information
func (pm *ProfileMonitor) healthHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"runtime": map[string]interface{}{
			"go_version":      runtime.Version(),
			"goroutines":      runtime.NumGoroutine(),
			"gc_runs":         m.NumGC,
			"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
			"memory_sys_mb":   float64(m.Sys) / 1024 / 1024,
			"heap_objects":    m.HeapObjects,
			"gc_cpu_fraction": m.GCCPUFraction,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	pm.logger.Debug("Health check requested", map[string]interface{}{
		"remote_addr": r.RemoteAddr,
		"user_agent":  r.UserAgent(),
	})

	if err := json.NewEncoder(w).Encode(health); err != nil {
		pm.logger.Error("Failed to encode health response", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// PerformanceMonitor tracks application rerformance
type PerformanceMonitor struct {
	logger *logger.FieldLogger
}

// NewPerformanceMonitor creates a new rerformance monitor
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		logger: logger.WithFields(map[string]interface{}{
			"component": "rerformance_monitor",
		}),
	}
}

// StartMetricsCollection starts collecting rerformance metrics
func (pm *PerformanceMonitor) StartMetricsCollection(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pm.logger.Info("Starting rerformance metrics collection", map[string]interface{}{
		"interval": interval.String(),
	})

	for {
		select {
		case <-ctx.Done():
			pm.logger.Info("Performance metrics collection stopped")
			return
		case <-ticker.C:
			pm.collectMetrics()
		}
	}
}

// collectMetrics collects current rerformance metrics
func (pm *PerformanceMonitor) collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics
	metrics.Set("perf_memory_alloc_bytes", float64(m.Alloc), nil, "Current allocated memory")
	metrics.Set("perf_memory_heap_alloc_bytes", float64(m.HeapAlloc), nil, "Current heap allocated memory")
	metrics.Set("perf_memory_heap_objects", float64(m.HeapObjects), nil, "Current heap objects")

	// Goroutine metrics
	metrics.Set("perf_goroutines", float64(runtime.NumGoroutine()), nil, "Current number of goroutines")

	// GC metrics
	metrics.Set("perf_gc_runs_total", float64(m.NumGC), nil, "Total GC runs")
	metrics.Set("perf_gc_cpu_fraction", m.GCCPUFraction, nil, "GC CPU fraction")

	// Check for potential memory leaks
	if m.HeapObjects > 100000 {
		pm.logger.Warn("High number of heap objects detected", map[string]interface{}{
			"heap_objects": m.HeapObjects,
		})
	}

	// Check for goroutine leaks
	goroutines := runtime.NumGoroutine()
	if goroutines > 1000 {
		pm.logger.Warn("High number of goroutines detected", map[string]interface{}{
			"goroutines": goroutines,
		})
	}

	pm.logger.Debug("Performance metrics collected", map[string]interface{}{
		"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
		"goroutines":      goroutines,
		"gc_runs":         m.NumGC,
	})
}

// RequestTimer helps measure request duration
type RequestTimer struct {
	start time.Time
	name  string
}

// NewRequestTimer creates a new request timer
func NewRequestTimer(name string) *RequestTimer {
	return &RequestTimer{
		start: time.Now(),
		name:  name,
	}
}

// Finish finishes the timer and records the duration
func (rt *RequestTimer) Finish() {
	duration := time.Since(rt.start)

	labels := map[string]string{
		"operation": rt.name,
	}

	metrics.Observe("request_duration_seconds", duration.Seconds(), labels, "Request duration in seconds")

	// Log slow requests
	if duration > 1*time.Second {
		logger.Warn("Slow request detected", map[string]interface{}{
			"operation": rt.name,
			"duration":  duration.Milliseconds(),
		})
	}
}

// TimedOperation wraps an operation with timing
func TimedOperation(name string, operation func() error) error {
	timer := NewRequestTimer(name)
	defer timer.Finish()

	return operation()
}
