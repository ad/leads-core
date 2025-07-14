package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// MetricType represents the type of metric
type MetricType string

const (
	Counter   MetricType = "counter"
	Gauge     MetricType = "gauge"
	Histogram MetricType = "histogram"
)

// Metric represents a single metric
type Metric struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Help      string                 `json:"help,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// MetricsCollector collects and manages metrics
type MetricsCollector struct {
	mu        sync.RWMutex
	metrics   map[string]*Metric
	startTime time.Time
}

// NewCollector creates a new metrics collector
func NewCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics:   make(map[string]*Metric),
		startTime: time.Now(),
	}
}

// metricKey generates a unique key for a metric
func (mc *MetricsCollector) metricKey(name string, labels map[string]string) string {
	key := name
	if len(labels) > 0 {
		labelStr := ""
		for k, v := range labels {
			if labelStr != "" {
				labelStr += ","
			}
			labelStr += fmt.Sprintf("%s=%s", k, v)
		}
		key += "{" + labelStr + "}"
	}
	return key
}

// Inc increments a counter metric
func (mc *MetricsCollector) Inc(name string, labels map[string]string, help string) {
	mc.Add(name, 1, labels, help)
}

// Add adds a value to a counter metric
func (mc *MetricsCollector) Add(name string, value float64, labels map[string]string, help string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		metric.Value += value
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      Counter,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
			Help:      help,
		}
	}
}

// Set sets a gauge metric value
func (mc *MetricsCollector) Set(name string, value float64, labels map[string]string, help string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	mc.metrics[key] = &Metric{
		Name:      name,
		Type:      Gauge,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
		Help:      help,
	}
}

// Observe records a histogram observation
func (mc *MetricsCollector) Observe(name string, value float64, labels map[string]string, help string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		// For histograms, we store additional statistics
		if metric.Extra == nil {
			metric.Extra = make(map[string]interface{})
		}

		count, _ := metric.Extra["count"].(float64)
		sum, _ := metric.Extra["sum"].(float64)
		min, _ := metric.Extra["min"].(float64)
		max, _ := metric.Extra["max"].(float64)

		count++
		sum += value
		if count == 1 || value < min {
			min = value
		}
		if count == 1 || value > max {
			max = value
		}

		metric.Extra["count"] = count
		metric.Extra["sum"] = sum
		metric.Extra["min"] = min
		metric.Extra["max"] = max
		metric.Extra["avg"] = sum / count
		metric.Value = sum / count // Store average as main value
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      Histogram,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
			Help:      help,
			Extra: map[string]interface{}{
				"count": 1.0,
				"sum":   value,
				"min":   value,
				"max":   value,
				"avg":   value,
			},
		}
	}
}

// GetMetrics returns all collected metrics
func (mc *MetricsCollector) GetMetrics() map[string]*Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*Metric)
	for k, v := range mc.metrics {
		metric := *v // Copy the metric
		if v.Extra != nil {
			metric.Extra = make(map[string]interface{})
			for ek, ev := range v.Extra {
				metric.Extra[ek] = ev
			}
		}
		result[k] = &metric
	}
	return result
}

// collectSystemMetrics collects system-level metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics
	mc.Set("system_memory_alloc_bytes", float64(m.Alloc), nil, "Currently allocated memory in bytes")
	mc.Set("system_memory_total_alloc_bytes", float64(m.TotalAlloc), nil, "Total allocated memory in bytes")
	mc.Set("system_memory_sys_bytes", float64(m.Sys), nil, "System memory in bytes")
	mc.Set("system_memory_heap_alloc_bytes", float64(m.HeapAlloc), nil, "Heap allocated memory in bytes")
	mc.Set("system_memory_heap_sys_bytes", float64(m.HeapSys), nil, "Heap system memory in bytes")
	mc.Set("system_memory_heap_objects", float64(m.HeapObjects), nil, "Number of heap objects")

	// GC metrics
	mc.Set("system_gc_num", float64(m.NumGC), nil, "Number of garbage collections")
	mc.Set("system_gc_cpu_fraction", m.GCCPUFraction, nil, "GC CPU fraction")

	// Goroutine metrics
	mc.Set("system_goroutines", float64(runtime.NumGoroutine()), nil, "Number of goroutines")

	// Uptime
	mc.Set("system_uptime_seconds", time.Since(mc.startTime).Seconds(), nil, "System uptime in seconds")
}

// GetSystemMetrics returns system metrics
func (mc *MetricsCollector) GetSystemMetrics() map[string]*Metric {
	mc.collectSystemMetrics()
	return mc.GetMetrics()
}

// HTTPMetricsMiddleware creates middleware for collecting HTTP metrics
func (mc *MetricsCollector) HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		labels := map[string]string{
			"method": r.Method,
			"path":   r.URL.Path,
			"status": fmt.Sprintf("%d", wrapped.statusCode),
		}

		mc.Inc("http_requests_total", labels, "Total HTTP requests")
		mc.Observe("http_request_duration_seconds", duration, labels, "HTTP request duration in seconds")

		// Record status code specific metrics
		statusLabels := map[string]string{
			"status": fmt.Sprintf("%d", wrapped.statusCode),
		}
		mc.Inc("http_responses_total", statusLabels, "Total HTTP responses by status code")
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsHandler returns an HTTP handler for exposing metrics
func (mc *MetricsCollector) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := mc.GetSystemMetrics()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"timestamp": time.Now(),
			"metrics":   metrics,
		})
	}
}

// Global metrics collector
var defaultCollector *MetricsCollector

// Init initializes the global metrics collector
func Init() {
	defaultCollector = NewCollector()
}

// Global metric functions
func Inc(name string, labels map[string]string, help string) {
	if defaultCollector != nil {
		defaultCollector.Inc(name, labels, help)
	}
}

func Add(name string, value float64, labels map[string]string, help string) {
	if defaultCollector != nil {
		defaultCollector.Add(name, value, labels, help)
	}
}

func Set(name string, value float64, labels map[string]string, help string) {
	if defaultCollector != nil {
		defaultCollector.Set(name, value, labels, help)
	}
}

func Observe(name string, value float64, labels map[string]string, help string) {
	if defaultCollector != nil {
		defaultCollector.Observe(name, value, labels, help)
	}
}

func HTTPMiddleware(next http.Handler) http.Handler {
	if defaultCollector != nil {
		return defaultCollector.HTTPMetricsMiddleware(next)
	}
	return next
}

func Handler() http.HandlerFunc {
	if defaultCollector != nil {
		return defaultCollector.MetricsHandler()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Metrics not initialized", http.StatusInternalServerError)
	}
}

func GetMetrics() map[string]*Metric {
	if defaultCollector != nil {
		return defaultCollector.GetMetrics()
	}
	return nil
}
