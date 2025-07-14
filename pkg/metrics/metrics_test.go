package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsCollector_Counter(t *testing.T) {
	collector := NewCollector()

	// Test increment
	collector.Inc("test_counter", map[string]string{"label": "value"}, "Test counter")

	metrics := collector.GetMetrics()
	key := "test_counter{label=value}"

	if metric, exists := metrics[key]; exists {
		if metric.Type != Counter {
			t.Errorf("Expected type Counter, got %s", metric.Type)
		}
		if metric.Value != 1 {
			t.Errorf("Expected value 1, got %f", metric.Value)
		}
		if metric.Name != "test_counter" {
			t.Errorf("Expected name test_counter, got %s", metric.Name)
		}
	} else {
		t.Errorf("Counter metric not found")
	}

	// Test add
	collector.Add("test_counter", 5, map[string]string{"label": "value"}, "Test counter")

	metrics = collector.GetMetrics()
	if metric, exists := metrics[key]; exists {
		if metric.Value != 6 { // 1 + 5
			t.Errorf("Expected value 6, got %f", metric.Value)
		}
	} else {
		t.Errorf("Counter metric not found after add")
	}
}

func TestMetricsCollector_Gauge(t *testing.T) {
	collector := NewCollector()

	collector.Set("test_gauge", 42.5, map[string]string{"type": "test"}, "Test gauge")

	metrics := collector.GetMetrics()
	key := "test_gauge{type=test}"

	if metric, exists := metrics[key]; exists {
		if metric.Type != Gauge {
			t.Errorf("Expected type Gauge, got %s", metric.Type)
		}
		if metric.Value != 42.5 {
			t.Errorf("Expected value 42.5, got %f", metric.Value)
		}
	} else {
		t.Errorf("Gauge metric not found")
	}

	// Test set again (should overwrite)
	collector.Set("test_gauge", 100, map[string]string{"type": "test"}, "Test gauge")

	metrics = collector.GetMetrics()
	if metric, exists := metrics[key]; exists {
		if metric.Value != 100 {
			t.Errorf("Expected value 100, got %f", metric.Value)
		}
	}
}

func TestMetricsCollector_Histogram(t *testing.T) {
	collector := NewCollector()

	// Add multiple observations
	collector.Observe("test_histogram", 10, nil, "Test histogram")
	collector.Observe("test_histogram", 20, nil, "Test histogram")
	collector.Observe("test_histogram", 30, nil, "Test histogram")

	metrics := collector.GetMetrics()
	key := "test_histogram"

	if metric, exists := metrics[key]; exists {
		if metric.Type != Histogram {
			t.Errorf("Expected type Histogram, got %s", metric.Type)
		}

		// Check extra fields
		if count, ok := metric.Extra["count"].(float64); !ok || count != 3 {
			t.Errorf("Expected count 3, got %v", metric.Extra["count"])
		}
		if sum, ok := metric.Extra["sum"].(float64); !ok || sum != 60 {
			t.Errorf("Expected sum 60, got %v", metric.Extra["sum"])
		}
		if min, ok := metric.Extra["min"].(float64); !ok || min != 10 {
			t.Errorf("Expected min 10, got %v", metric.Extra["min"])
		}
		if max, ok := metric.Extra["max"].(float64); !ok || max != 30 {
			t.Errorf("Expected max 30, got %v", metric.Extra["max"])
		}
		if avg, ok := metric.Extra["avg"].(float64); !ok || avg != 20 {
			t.Errorf("Expected avg 20, got %v", metric.Extra["avg"])
		}
	} else {
		t.Errorf("Histogram metric not found")
	}
}

func TestMetricsCollector_SystemMetrics(t *testing.T) {
	collector := NewCollector()

	metrics := collector.GetSystemMetrics()

	// Check that system metrics are present
	expectedMetrics := []string{
		"system_memory_alloc_bytes",
		"system_goroutines",
		"system_uptime_seconds",
	}

	for _, expected := range expectedMetrics {
		if _, exists := metrics[expected]; !exists {
			t.Errorf("Expected system metric %s not found", expected)
		}
	}

	// Check that uptime is reasonable (should be very small since we just created the collector)
	if uptime, exists := metrics["system_uptime_seconds"]; exists {
		if uptime.Value < 0 || uptime.Value > 1 {
			t.Errorf("Unexpected uptime value: %f", uptime.Value)
		}
	}
}

func TestHTTPMetricsMiddleware(t *testing.T) {
	collector := NewCollector()

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Small delay to test duration
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with metrics middleware
	wrappedHandler := collector.HTTPMetricsMiddleware(handler)

	// Create test request
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record response
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status OK, got %v", status)
	}

	// Check metrics
	metrics := collector.GetMetrics()

	// Debug: print all metric keys
	found := false
	for key := range metrics {
		if key == "http_requests_total{method=GET,path=/test,status=200}" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected request count metric not found")
	}

	// Should have duration metric - check for existence with any key containing the base name
	durationFound := false
	for _, metric := range metrics {
		if metric.Name == "http_request_duration_seconds" && metric.Type == Histogram {
			durationFound = true
			// Duration should be > 0.01 seconds (10ms)
			if metric.Value < 0.01 {
				t.Errorf("Expected duration >= 0.01s, got %f", metric.Value)
			}
			break
		}
	}

	if !durationFound {
		t.Errorf("Expected duration metric not found")
	}
}

func TestMetricsHandler(t *testing.T) {
	collector := NewCollector()

	// Add some test metrics
	collector.Inc("test_requests", nil, "Test requests")
	collector.Set("test_gauge", 42, nil, "Test gauge")

	// Test metrics handler
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := collector.MetricsHandler()
	handler.ServeHTTP(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status OK, got %v", status)
	}

	// Check content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse metrics response: %v", err)
	}

	// Check structure
	if _, exists := response["timestamp"]; !exists {
		t.Errorf("Expected timestamp in response")
	}
	if _, exists := response["metrics"]; !exists {
		t.Errorf("Expected metrics in response")
	}
}

func TestGlobalMetrics(t *testing.T) {
	// Initialize global metrics
	Init()

	// Test global functions
	Inc("global_counter", nil, "Global counter")
	Set("global_gauge", 100, nil, "Global gauge")
	Add("global_counter", 5, nil, "Global counter")
	Observe("global_histogram", 1.5, nil, "Global histogram")

	// Test HTTP middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := HTTPMiddleware(handler)
	req, _ := http.NewRequest("GET", "/global-test", nil)
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	// Test metrics handler
	metricsHandler := Handler()
	req, _ = http.NewRequest("GET", "/metrics", nil)
	rr = httptest.NewRecorder()
	metricsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Global metrics handler failed")
	}
}

func TestMetricKey(t *testing.T) {
	collector := NewCollector()

	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "test_metric",
			labels:   nil,
			expected: "test_metric",
		},
		{
			name:     "test_metric",
			labels:   map[string]string{"key": "value"},
			expected: "test_metric{key=value}",
		},
		{
			name:     "test_metric",
			labels:   map[string]string{"key1": "value1", "key2": "value2"},
			expected: "test_metric{key1=value1,key2=value2}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			key := collector.metricKey(tt.name, tt.labels)

			// For multiple labels, order might vary, so we check both possibilities
			if len(tt.labels) <= 1 {
				if key != tt.expected {
					t.Errorf("Expected key %s, got %s", tt.expected, key)
				}
			} else {
				// For multiple labels, just check that it contains the metric name and labels
				if !containsAll(key, []string{"test_metric", "key1=value1", "key2=value2"}) {
					t.Errorf("Key %s doesn't contain expected parts", key)
				}
			}
		})
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !contains(s, part) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
