package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/ad/leads-core/internal/validation"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

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
	authMiddleware := middleware.NewAuthMiddleware(jwtValidator)

	// Create router
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
	mux.HandleFunc("/api/widgets/", func(w http.ResponseWriter, r *http.Request) {
		// Extract widget ID from path
		path := r.URL.Path
		if len(path) <= len("/api/widgets/") {
			http.Error(w, "Widget ID is required", http.StatusBadRequest)
			return
		}

		// Remove /api/widgets/ prefix and parse the rest
		remaining := path[len("/api/widgets/"):]

		// Check if it's a submission request
		if r.Method == "POST" && len(remaining) > 7 && remaining[len(remaining)-7:] == "/submit" {
			widgetID := remaining[:len(remaining)-7]
			handlePublicSubmission(w, r, widgetID, redisClient)
			return
		}

		// Get widget for viewing (public endpoint)
		if r.Method == "GET" {
			widgetID := remaining
			// Remove trailing slash if present
			if len(widgetID) > 0 && widgetID[len(widgetID)-1] == '/' {
				widgetID = widgetID[:len(widgetID)-1]
			}
			handleGetWidget(w, widgetID, redisClient)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// Private API endpoints (require authentication)
	mux.Handle("/api/private/", authMiddleware.RequireAuth(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract path after /api/private/
			path := r.URL.Path[len("/api/private/"):]

			switch {
			case path == "widgets" && r.Method == "POST":
				handleCreateWidget(w, r, redisClient)
			case path == "widgets" && r.Method == "GET":
				handleListWidgets(w, r, redisClient)
			case len(path) > 6 && path[:6] == "widgets/":
				widgetID := path[6:]
				// Remove potential trailing slash or path suffixes
				if idx := strings.Index(widgetID, "/"); idx > 0 {
					suffix := widgetID[idx:]
					widgetID = widgetID[:idx]

					if suffix == "/stats" && r.Method == "GET" {
						handleGetStats(w, r, widgetID, redisClient)
						return
					}
				}

				switch r.Method {
				case "GET":
					handleGetPrivateWidget(w, r, widgetID, redisClient)
				case "PUT":
					handleUpdateWidget(w, r, widgetID, redisClient)
				case "DELETE":
					handleDeleteWidget(w, r, widgetID, redisClient)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			default:
				http.Error(w, "Not found", http.StatusNotFound)
			}
		}),
	))

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

	// Set default headers
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// Test handlers implementation (simplified versions for E2E testing)

func handlePublicSubmission(w http.ResponseWriter, r *http.Request, widgetID string, client redis.UniversalClient) {
	// Parse request body first
	var submission struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Check if data field exists
	if len(submission.Data) == 0 {
		http.Error(w, "Submission data is required", http.StatusBadRequest)
		return
	}

	// Check if widget exists and is enabled
	ctx := context.Background()
	widgetData, err := client.HGetAll(ctx, "widget:"+widgetID).Result()
	if err != nil || len(widgetData) == 0 {
		http.Error(w, "Widget not found", http.StatusNotFound)
		return
	}

	if widgetData["enabled"] != "true" {
		http.Error(w, "Widget is disabled", http.StatusBadRequest)
		return
	}

	// Create submission
	submissionID := fmt.Sprintf("sub_%d", time.Now().UnixNano())
	submissionKey := fmt.Sprintf("submission:%s:%s", widgetID, submissionID)

	submissionObj := map[string]interface{}{
		"id":         submissionID,
		"widget_id":  widgetID,
		"data":       submission.Data,
		"created_at": time.Now().Format(time.RFC3339),
	}

	submissionJSON, _ := json.Marshal(submissionObj)

	// Store submission
	if err := client.HSet(ctx, submissionKey, "data", submissionJSON).Err(); err != nil {
		http.Error(w, "Failed to store submission", http.StatusInternalServerError)
		return
	}

	// Update widget stats
	client.HIncrBy(ctx, "widget:"+widgetID+":stats", "submits", 1)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":      submissionID,
		"message": "Submission created successfully",
	})
}

func handleGetWidget(w http.ResponseWriter, widgetID string, client redis.UniversalClient) {
	ctx := context.Background()
	widgetData, err := client.HGetAll(ctx, "widget:"+widgetID).Result()
	if err != nil || len(widgetData) == 0 {
		http.Error(w, "Widget not found", http.StatusNotFound)
		return
	}

	// Increment views
	client.HIncrBy(ctx, "widget:"+widgetID+":stats", "views", 1)

	// Parse fields JSON
	var fields map[string]interface{}
	if fieldsStr, ok := widgetData["fields"]; ok {
		json.Unmarshal([]byte(fieldsStr), &fields)
	}

	// Return only public widget data
	publicWidget := map[string]interface{}{
		"id":      widgetID,
		"name":    widgetData["name"],
		"type":    widgetData["type"],
		"enabled": widgetData["enabled"] == "true",
		"fields":  fields,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(publicWidget)
}

func handleCreateWidget(w http.ResponseWriter, r *http.Request, client redis.UniversalClient) {
	// Get user from context
	user, exists := auth.GetUserFromContext(r.Context())
	if !exists {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	// Parse request first
	var widget models.Widget
	if err := json.NewDecoder(r.Body).Decode(&widget); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Basic validation - check required fields
	if widget.Name == "" {
		http.Error(w, "Widget name is required", http.StatusBadRequest)
		return
	}
	if widget.Type == "" {
		http.Error(w, "Widget type is required", http.StatusBadRequest)
		return
	}

	// Set widget metadata
	widget.ID = fmt.Sprintf("widget_%d", time.Now().UnixNano())
	widget.OwnerID = user.ID
	widget.CreatedAt = time.Now()
	widget.UpdatedAt = time.Now()

	// Store widget in Redis
	ctx := context.Background()
	widgetKey := "widget:" + widget.ID

	fieldsJSON, _ := json.Marshal(widget.Fields)

	widgetData := map[string]interface{}{
		"id":         widget.ID,
		"owner_id":   widget.OwnerID,
		"name":       widget.Name,
		"type":       widget.Type,
		"enabled":    fmt.Sprintf("%v", widget.Enabled),
		"fields":     string(fieldsJSON),
		"created_at": widget.CreatedAt.Format(time.RFC3339),
		"updated_at": widget.UpdatedAt.Format(time.RFC3339),
	}

	if err := client.HMSet(ctx, widgetKey, widgetData).Err(); err != nil {
		http.Error(w, "Failed to create widget", http.StatusInternalServerError)
		return
	}

	// Add to user's widgets set
	client.SAdd(ctx, "widgets:"+user.ID, widget.ID)

	// Initialize stats
	client.HMSet(ctx, "widget:"+widget.ID+":stats", map[string]interface{}{
		"widget_id": widget.ID,
		"views":     0,
		"submits":   0,
		"closes":    0,
	})

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(widget)
}

func handleListWidgets(w http.ResponseWriter, r *http.Request, client redis.UniversalClient) {
	user, exists := auth.GetUserFromContext(r.Context())
	if !exists {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	widgetIDs, err := client.SMembers(ctx, "widgets:"+user.ID).Result()
	if err != nil {
		http.Error(w, "Failed to get widgets", http.StatusInternalServerError)
		return
	}

	var widgets []map[string]interface{}
	for _, widgetID := range widgetIDs {
		widgetData, err := client.HGetAll(ctx, "widget:"+widgetID).Result()
		if err != nil || len(widgetData) == 0 {
			continue
		}

		widgets = append(widgets, map[string]interface{}{
			"id":      widgetData["id"],
			"name":    widgetData["name"],
			"type":    widgetData["type"],
			"enabled": widgetData["enabled"] == "true",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": widgets,
		"meta": map[string]interface{}{
			"total": len(widgets),
		},
	})
}

func handleGetPrivateWidget(w http.ResponseWriter, r *http.Request, widgetID string, client redis.UniversalClient) {
	user, exists := auth.GetUserFromContext(r.Context())
	if !exists {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	widgetData, err := client.HGetAll(ctx, "widget:"+widgetID).Result()
	if err != nil || len(widgetData) == 0 {
		http.Error(w, "Widget not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if widgetData["owner_id"] != user.ID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Parse fields from JSON string
	var fields []map[string]interface{}
	if fieldsStr, ok := widgetData["fields"]; ok && fieldsStr != "" {
		if err := json.Unmarshal([]byte(fieldsStr), &fields); err != nil {
			fields = []map[string]interface{}{}
		}
	}

	// Parse enabled field
	enabled := false
	if enabledStr, ok := widgetData["enabled"]; ok {
		enabled = enabledStr == "true"
	}

	// Create response with properly typed fields
	response := map[string]interface{}{
		"id":         widgetData["id"],
		"name":       widgetData["name"],
		"type":       widgetData["type"],
		"fields":     fields,
		"enabled":    enabled,
		"owner_id":   widgetData["owner_id"],
		"created_at": widgetData["created_at"],
		"updated_at": widgetData["updated_at"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": response})
}

func handleUpdateWidget(w http.ResponseWriter, r *http.Request, widgetID string, client redis.UniversalClient) {
	user, exists := auth.GetUserFromContext(r.Context())
	if !exists {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	// Check widget ownership
	ctx := context.Background()
	ownerID, err := client.HGet(ctx, "widget:"+widgetID, "owner_id").Result()
	if err != nil {
		http.Error(w, "Widget not found", http.StatusNotFound)
		return
	}

	if ownerID != user.ID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Parse request first
	var updateData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Basic validation - don't allow empty name if it's being updated
	if name, exists := updateData["name"]; exists {
		if nameStr, ok := name.(string); ok && nameStr == "" {
			http.Error(w, "Widget name cannot be empty", http.StatusBadRequest)
			return
		}
	}

	// Process special fields that need conversion
	if enabled, exists := updateData["enabled"]; exists {
		updateData["enabled"] = fmt.Sprintf("%v", enabled)
	}

	// Update widget
	updateData["updated_at"] = time.Now().Format(time.RFC3339)
	if err := client.HMSet(ctx, "widget:"+widgetID, updateData).Err(); err != nil {
		http.Error(w, "Failed to update widget", http.StatusInternalServerError)
		return
	}

	// Get updated widget
	widgetData, _ := client.HGetAll(ctx, "widget:"+widgetID).Result()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(widgetData)
}

func handleDeleteWidget(w http.ResponseWriter, r *http.Request, widgetID string, client redis.UniversalClient) {
	user, exists := auth.GetUserFromContext(r.Context())
	if !exists {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	// Check widget ownership
	ctx := context.Background()
	ownerID, err := client.HGet(ctx, "widget:"+widgetID, "owner_id").Result()
	if err != nil {
		http.Error(w, "Widget not found", http.StatusNotFound)
		return
	}

	if ownerID != user.ID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Delete widget and related data
	client.Del(ctx, "widget:"+widgetID)
	client.Del(ctx, "widget:"+widgetID+":stats")
	client.SRem(ctx, "widgets:"+user.ID, widgetID)

	w.WriteHeader(http.StatusNoContent)
}

func handleGetStats(w http.ResponseWriter, r *http.Request, widgetID string, client redis.UniversalClient) {
	user, exists := auth.GetUserFromContext(r.Context())
	if !exists {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	// Check widget ownership
	ctx := context.Background()
	ownerID, err := client.HGet(ctx, "widget:"+widgetID, "owner_id").Result()
	if err != nil {
		http.Error(w, "Widget not found", http.StatusNotFound)
		return
	}

	if ownerID != user.ID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Get stats
	stats, err := client.HGetAll(ctx, "widget:"+widgetID+":stats").Result()
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// E2E Test Cases

func TestE2E_HealthCheck(t *testing.T) {
	e2e := setupE2EServer(t)

	resp, err := e2e.makeRequest("GET", "/health", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", health["status"])
	}
}

func TestE2E_WidgetLifecycle(t *testing.T) {
	e2e := setupE2EServer(t)
	userID := "test-user-e2e"
	token := e2e.createTestToken(userID)
	headers := map[string]string{
		"Authorization": "Bearer " + token,
	}

	// Step 1: Create a widget
	createWidgetData := []byte(`{
		"name": "E2E Test Widget",
		"type": "contact",
		"enabled": true,
		"fields": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/private/widgets", createWidgetData, headers)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
		// Read response body for debugging
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		t.Logf("Response body: %s", buf.String())
		return
	}

	var createdWidget models.Widget
	if err := json.NewDecoder(resp.Body).Decode(&createdWidget); err != nil {
		t.Fatalf("Failed to decode widget: %v", err)
	}

	widgetID := createdWidget.ID
	if widgetID == "" {
		t.Fatal("Widget ID is empty")
	}

	// Step 2: List widgets
	resp, err = e2e.makeRequest("GET", "/api/private/widgets", nil, headers)
	if err != nil {
		t.Fatalf("Failed to list widgets: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var listResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}

	widgets, ok := listResponse["data"].([]interface{})
	if !ok || len(widgets) == 0 {
		t.Error("Expected at least one widget in list")
	}

	// Step 3: Get widget details
	resp, err = e2e.makeRequest("GET", "/api/private/widgets/"+widgetID, nil, headers)
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
		"enabled": false
	}`)

	resp, err = e2e.makeRequest("PUT", "/api/private/widgets/"+widgetID, updateData, headers)
	if err != nil {
		t.Fatalf("Failed to update widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 5: Delete widget
	resp, err = e2e.makeRequest("DELETE", "/api/private/widgets/"+widgetID, nil, headers)
	if err != nil {
		t.Fatalf("Failed to delete widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	// Step 6: Verify widget is deleted
	resp, err = e2e.makeRequest("GET", "/api/private/widgets/"+widgetID, nil, headers)
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
	userID := "test-user-public"
	token := e2e.createTestToken(userID)
	headers := map[string]string{
		"Authorization": "Bearer " + token,
	}

	// Create a widget first
	createWidgetData := []byte(`{
		"name": "Public Submission Widget",
		"type": "contact",
		"enabled": true,
		"fields": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/private/widgets", createWidgetData, headers)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}
	defer resp.Body.Close()

	var widget models.Widget
	if err := json.NewDecoder(resp.Body).Decode(&widget); err != nil {
		t.Fatalf("Failed to decode widget: %v", err)
	}

	widgetID := widget.ID

	// Test public widget view
	resp, err = e2e.makeRequest("GET", "/api/widgets/"+widgetID, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get public widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test public submission
	submissionData := []byte(`{
		"data": {
			"name": "John Doe",
			"email": "john@example.com"
		}
	}`)

	resp, err = e2e.makeRequest("POST", "/api/widgets/"+widgetID+"/submit", submissionData, nil)
	if err != nil {
		t.Fatalf("Failed to submit widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var submissionResponse map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&submissionResponse); err != nil {
		t.Fatalf("Failed to decode submission response: %v", err)
	}

	if submissionResponse["id"] == "" {
		t.Error("Submission ID is empty")
	}

	// Check stats
	resp, err = e2e.makeRequest("GET", "/api/private/widgets/"+widgetID+"/stats", nil, headers)
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

	// Should have at least 1 view and 1 submit
	if stats["views"] == "0" {
		t.Error("Expected at least 1 view")
	}
	if stats["submits"] == "0" {
		t.Error("Expected at least 1 submission")
	}
}

func TestE2E_Authorization(t *testing.T) {
	e2e := setupE2EServer(t)
	user1ID := "user1"
	user2ID := "user2"
	token1 := e2e.createTestToken(user1ID)
	token2 := e2e.createTestToken(user2ID)

	headers1 := map[string]string{"Authorization": "Bearer " + token1}
	headers2 := map[string]string{"Authorization": "Bearer " + token2}

	// User 1 creates a widget
	createWidgetData := []byte(`{
		"name": "User 1 Widget",
		"type": "contact",
		"enabled": true,
		"fields": {"name": {"type": "text"}}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/private/widgets", createWidgetData, headers1)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
	}
	defer resp.Body.Close()

	var widget models.Widget
	json.NewDecoder(resp.Body).Decode(&widget)
	widgetID := widget.ID

	// User 2 tries to access User 1's widget (should fail)
	resp, err = e2e.makeRequest("GET", "/api/private/widgets/"+widgetID, nil, headers2)
	if err != nil {
		t.Fatalf("Failed to request widget: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}

	// User 2 tries to update User 1's widget (should fail)
	updateData := []byte(`{"name": "Hacked Widget"}`)
	resp, err = e2e.makeRequest("PUT", "/api/private/widgets/"+widgetID, updateData, headers2)
	if err != nil {
		t.Fatalf("Failed to request update: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}

	// User 2 tries to delete User 1's widget (should fail)
	resp, err = e2e.makeRequest("DELETE", "/api/private/widgets/"+widgetID, nil, headers2)
	if err != nil {
		t.Fatalf("Failed to request delete: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}
}

func TestE2E_InvalidRequests(t *testing.T) {
	e2e := setupE2EServer(t)
	userID := "test-user-invalid"
	token := e2e.createTestToken(userID)
	headers := map[string]string{"Authorization": "Bearer " + token}

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
			path:           "/api/private/widgets",
			body:           []byte(`{invalid json`),
			headers:        headers,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing required fields",
			method:         "POST",
			path:           "/api/private/widgets",
			body:           []byte(`{"type": "contact"}`),
			headers:        headers,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized request",
			method:         "POST",
			path:           "/api/private/widgets",
			body:           []byte(`{"name": "Test", "type": "contact"}`),
			headers:        nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "nonexistent widget",
			method:         "GET",
			path:           "/api/private/widgets/nonexistent",
			body:           nil,
			headers:        headers,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid submission - missing data",
			method:         "POST",
			path:           "/api/widgets/nonexistent/submit",
			body:           []byte(`{"invalid": "data"}`),
			headers:        nil,
			expectedStatus: http.StatusBadRequest,
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
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}
