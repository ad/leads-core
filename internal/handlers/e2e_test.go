package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/auth"
	"github.com/ad/leads-core/internal/config"
	"github.com/ad/leads-core/internal/middleware"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/storage"
	"github.com/ad/leads-core/internal/validation"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// routePrivateWidgetEndpoints routes private widget endpoints for /api/v1/widgets/*
func routePrivateWidgetEndpoints(handler *WidgetHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Remove /api/v1/widgets prefix to get the actual path
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/widgets")

		switch {
		case path == "" || path == "/":
			// GET /api/v1/widgets - list widgets
			// POST /api/v1/widgets - create widget
			switch r.Method {
			case http.MethodGet:
				handler.GetWidgets(w, r)
			case http.MethodPost:
				handler.CreateWidget(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case path == "/summary":
			// GET /api/v1/widgets/summary
			if r.Method == http.MethodGet {
				handler.GetWidgetsSummary(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case strings.HasSuffix(path, "/stats"):
			// GET /api/v1/widgets/{id}/stats
			// Reconstruct URL as /widgets/{id}/stats for handler
			r.URL.Path = "/widgets" + path
			handler.GetWidgetStats(w, r)
		case strings.HasSuffix(path, "/submissions"):
			// GET /api/v1/widgets/{id}/submissions
			// Reconstruct URL as /widgets/{id}/submissions for handler
			r.URL.Path = "/widgets" + path
			handler.GetWidgetSubmissions(w, r)
		case strings.HasSuffix(path, "/export"):
			// GET /api/v1/widgets/{id}/export
			// Reconstruct URL as /widgets/{id}/export for handler
			r.URL.Path = "/widgets" + path
			handler.ExportWidgetSubmissions(w, r)
		default:
			// GET /api/v1/widgets/{id} - get widget
			// POST /api/v1/widgets/{id} - update widget
			// PUT /api/v1/widgets/{id}/config - update widget configuration
			// DELETE /api/v1/widgets/{id} - delete widget
			// Reconstruct URL as /widgets/{id} for handler
			r.URL.Path = "/widgets" + path
			switch r.Method {
			case http.MethodGet:
				handler.GetWidget(w, r)
			case http.MethodPost:
				handler.UpdateWidget(w, r)
			case http.MethodPut:
				// Handle PUT for updating widget configuration
				if strings.HasSuffix(path, "/config") {
					handler.UpdateWidgetConfig(w, r)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case http.MethodDelete:
				handler.DeleteWidget(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	}
}

// routePublicWidgetEndpoints routes public widget endpoints
func routePublicWidgetEndpoints(handler *PublicHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/submit"):
			// POST /widgets/{id}/submit
			handler.SubmitWidget(w, r)
		case strings.HasSuffix(path, "/events"):
			// POST /widgets/{id}/events
			handler.RegisterEvent(w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}

// E2ETestServer represents a complete test server for end-to-end testing
type E2ETestServer struct {
	server      *httptest.Server
	redis       *miniredis.Miniredis
	redisClient redis.UniversalClient
	config      config.Config
	validator   *validation.SchemaValidator
	baseURL     string
}

// setupE2EServer creates a full HTTP server for end-to-end testing
func setupE2EServer(t *testing.T) *E2ETestServer {
	t.Helper()

	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create test config
	cfg := config.Config{
		Server: config.ServerConfig{
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		JWT: config.JWTConfig{
			Secret: "test-secret-key-for-e2e-tests",
		},
		RateLimit: config.RateLimitConfig{
			IPPerMinute:     100, // Higher limit for E2E tests
			GlobalPerMinute: 10000,
		},
		TTL: config.TTLConfig{
			DemoDays: 1,
			FreeDays: 30,
			ProDays:  365,
		},
	}

	// Create validator
	validator, err := validation.NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Create JWT validator
	jwtValidator := auth.NewJWTValidator(cfg.JWT.Secret)

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtValidator, false)

	// Create a RedisClient wrapper for the repositories
	wrappedRedisClient := storage.NewRedisClientWithUniversal(redisClient)

	// Initialize repositories
	statsRepo := storage.NewRedisStatsRepository(wrappedRedisClient)
	widgetRepo := storage.NewRedisWidgetRepository(wrappedRedisClient, statsRepo)
	submissionRepo := storage.NewRedisSubmissionRepository(wrappedRedisClient)

	// Initialize services
	ttlConfig := services.TTLConfig{
		DemoDays: cfg.TTL.DemoDays,
		FreeDays: cfg.TTL.FreeDays,
		ProDays:  cfg.TTL.ProDays,
	}
	widgetService := services.NewWidgetService(widgetRepo, submissionRepo, statsRepo, ttlConfig)
	exportService := services.NewExportService(submissionRepo, widgetRepo)

	// Initialize handlers
	widgetHandler := NewWidgetHandler(widgetService, exportService, validator)
	publicHandler := NewPublicHandler(widgetService, validator)

	// Create router using the same structure as main server
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now(),
			"services": map[string]string{
				"redis": "ok",
			},
		})
	})

	// Public widget submission endpoint (no auth required)
	publicChain := http.HandlerFunc(routePublicWidgetEndpoints(publicHandler))
	mux.Handle("/widgets/", publicChain)

	// Private API endpoints using the same routing as main server
	privateWidgetsChain := authMiddleware.Authenticate(http.HandlerFunc(routePrivateWidgetEndpoints(widgetHandler)))
	mux.Handle("/api/v1/widgets/", privateWidgetsChain)
	mux.Handle("/api/v1/widgets", privateWidgetsChain)

	// Start test server
	server := httptest.NewServer(mux)

	t.Cleanup(func() {
		server.Close()
		mr.Close()
		redisClient.Close()
	})

	return &E2ETestServer{
		server:      server,
		redis:       mr,
		redisClient: redisClient,
		config:      cfg,
		validator:   validator,
		baseURL:     server.URL,
	}
}

// createTestToken creates a valid JWT token for testing
func (e2e *E2ETestServer) createTestToken(userID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, _ := token.SignedString([]byte(e2e.config.JWT.Secret))
	return tokenString
}

// makeRequest makes an HTTP request to the test server
func (e2e *E2ETestServer) makeRequest(method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	url := e2e.baseURL + path

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// === E2E TESTS ===

func TestE2E_HealthCheck(t *testing.T) {
	e2e := setupE2EServer(t)

	resp, err := e2e.makeRequest("GET", "/health", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make health check request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}
}

func TestE2E_WidgetLifecycle(t *testing.T) {
	e2e := setupE2EServer(t)

	// Create test token
	token := e2e.createTestToken("test-user-id")
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	// Step 1: Create widget
	createWidgetData := []byte(`{
		"name": "E2E Test Widget",
		"type": "lead-form",
		"isVisible": true,
		"description": "Widget for end-to-end testing",
		"config": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/v1/widgets", createWidgetData, headers)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, body)
	}

	var widgetData models.Widget
	if err := json.NewDecoder(resp.Body).Decode(&widgetData); err != nil {
		t.Fatalf("Failed to decode created widget: %v", err)
	}

	if widgetData.ID == "" {
		t.Fatal("Widget ID is empty or not a string")
	}

	// Step 2: List widgets
	resp, err = e2e.makeRequest("GET", "/api/v1/widgets", nil, headers)
	if err != nil {
		t.Fatalf("Failed to list widgets: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 3: Get specific widget
	resp, err = e2e.makeRequest("GET", "/api/v1/widgets/"+widgetData.ID, nil, headers)
	if err != nil {
		t.Fatalf("Failed to get widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 4: Update widget
	updateData := []byte(`{
		"name": "Updated E2E Test Widget",
		"isVisible": false
	}`)

	resp, err = e2e.makeRequest("POST", "/api/v1/widgets/"+widgetData.ID, updateData, headers)
	if err != nil {
		t.Fatalf("Failed to update widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 5: Delete widget
	resp, err = e2e.makeRequest("DELETE", "/api/v1/widgets/"+widgetData.ID, nil, headers)
	if err != nil {
		t.Fatalf("Failed to delete widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	// Step 6: Verify widget is deleted
	resp, err = e2e.makeRequest("GET", "/api/v1/widgets/"+widgetData.ID, nil, headers)
	if err != nil {
		t.Fatalf("Failed to check deleted widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for deleted widget, got %d", resp.StatusCode)
	}
}

func TestE2E_PublicSubmission(t *testing.T) {
	e2e := setupE2EServer(t)

	// Create test token
	token := e2e.createTestToken("test-user-id")
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	// Step 1: Create widget
	createWidgetData := []byte(`{
		"name": "Submission Test Widget",
		"type": "lead-form",
		"isVisible": true,
		"description": "Widget for submission testing",
		"config": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true},
			"message": {"type": "textarea", "required": false}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/v1/widgets", createWidgetData, headers)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}
	defer resp.Body.Close()

	var widgetData models.Widget
	json.NewDecoder(resp.Body).Decode(&widgetData)

	if widgetData.ID == "" {
		t.Fatal("Widget ID is empty or not a string")
	}

	// Step 2: Submit data to widget (public endpoint, no auth)
	submissionData := []byte(`{
		"data": {
			"name": "John Doe",
			"email": "john@example.com",
			"message": "Test submission"
		}
	}`)

	publicHeaders := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err = e2e.makeRequest("POST", "/widgets/"+widgetData.ID+"/submit", submissionData, publicHeaders)
	if err != nil {
		t.Fatalf("Failed to submit data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201 for submission, got %d. Body: %s", resp.StatusCode, body)
	}

	// Step 3: Check stats
	resp, err = e2e.makeRequest("GET", "/api/v1/widgets/"+widgetData.ID+"/stats", nil, headers)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode stats: %v", err)
	}

	// Should have at least 1 submission
	if stats["submits"] == "0" {
		t.Error("Expected at least 1 submission")
	}
}

func TestE2E_Authorization(t *testing.T) {
	e2e := setupE2EServer(t)

	// Create test tokens for different users
	token1 := e2e.createTestToken("user1")
	token2 := e2e.createTestToken("user2")

	headers1 := map[string]string{
		"Authorization": "Bearer " + token1,
		"Content-Type":  "application/json",
	}

	headers2 := map[string]string{
		"Authorization": "Bearer " + token2,
		"Content-Type":  "application/json",
	}

	// Step 1: User1 creates widget
	createWidgetData := []byte(`{
		"name": "User1's Widget",
		"type": "lead-form",
		"isVisible": true,
		"config": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/v1/widgets", createWidgetData, headers1)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}
	defer resp.Body.Close()

	var widgetData models.Widget
	json.NewDecoder(resp.Body).Decode(&widgetData)

	if widgetData.ID == "" {
		t.Fatal("Widget ID is empty or not a string")
	}

	// Step 2: User2 tries to access User1's widget (should fail)
	resp, err = e2e.makeRequest("GET", "/api/v1/widgets/"+widgetData.ID, nil, headers2)
	if err != nil {
		t.Fatalf("Failed to get widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	// Step 3: User2 tries to update User1's widget (should fail)
	updateData := []byte(`{"name": "Hacked Widget"}`)

	resp, err = e2e.makeRequest("POST", "/api/v1/widgets/"+widgetData.ID, updateData, headers2)
	if err != nil {
		t.Fatalf("Failed to update widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	// Step 4: User2 tries to delete User1's widget (should fail)
	resp, err = e2e.makeRequest("DELETE", "/api/v1/widgets/"+widgetData.ID, nil, headers2)
	if err != nil {
		t.Fatalf("Failed to delete widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestE2E_InvalidRequests(t *testing.T) {
	e2e := setupE2EServer(t)

	token := e2e.createTestToken("test-user")
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	tests := []struct {
		name           string
		method         string
		path           string
		body           []byte
		headers        map[string]string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			method:         "POST",
			path:           "/api/v1/widgets",
			body:           []byte(`{"invalid": json}`),
			headers:        headers,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing required fields",
			method:         "POST",
			path:           "/api/v1/widgets",
			body:           []byte(`{"description": "Missing name and type"}`),
			headers:        headers,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized request",
			method:         "GET",
			path:           "/api/v1/widgets",
			body:           nil,
			headers:        map[string]string{"Content-Type": "application/json"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "nonexistent widget",
			method:         "GET",
			path:           "/api/v1/widgets/nonexistent",
			body:           nil,
			headers:        headers,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid submission - missing data",
			method:         "POST",
			path:           "/widgets/nonexistent/submit",
			body:           []byte(`{}`),
			headers:        map[string]string{"Content-Type": "application/json"},
			expectedStatus: http.StatusBadRequest, // Validation error happens before widget lookup
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := e2e.makeRequest(tt.method, tt.path, tt.body, tt.headers)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, body)
			}
		})
	}
}
