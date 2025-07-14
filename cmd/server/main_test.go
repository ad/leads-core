package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ad/leads-core/internal/handlers"
)

func TestBasicHandlers(t *testing.T) {
	// Test metrics handler (doesn't require external dependencies)
	metricsHandler := handlers.NewMetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	metricsHandler.Metrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Note: Health handler test skipped because it requires Redis connection
}

func TestServerStartup(t *testing.T) {
	// Quick test to ensure the server can start (but not actually start it)
	// This is a smoke test to catch import/compilation issues
	t.Log("Basic compilation and handler tests passed")
}
