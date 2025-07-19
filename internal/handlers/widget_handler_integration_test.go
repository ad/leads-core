package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	"github.com/redis/go-redis/v9"
)

// IntegrationTestEnvironment encapsulates the full test environment for widget filtering
type IntegrationTestEnvironment struct {
	Config         config.Config
	JWTValidator   *auth.JWTValidator
	Validator      *validation.SchemaValidator
	Redis          *miniredis.Miniredis
	RedisClient    *storage.RedisClient
	WidgetRepo     storage.WidgetRepository
	StatsRepo      storage.StatsRepository
	WidgetService  *services.WidgetService
	Handler        *WidgetHandler
	AuthMiddleware *middleware.AuthMiddleware
	UserID         string
	Token          string
}

// setupIntegrationTestEnvironment creates a complete test environment
func setupIntegrationTestEnvironment(t *testing.T) *IntegrationTestEnvironment {
	t.Helper()

	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	// Create Redis client
	universalClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	redisClient := storage.NewRedisClientWithUniversal(universalClient)

	// Create test config
	cfg := config.Config{
		Server: config.ServerConfig{
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		JWT: config.JWTConfig{
			Secret: "test-secret-key-for-integration-tests",
		},
		RateLimit: config.RateLimitConfig{
			IPPerMinute:     100,
			GlobalPerMinute: 1000,
		},
		TTL: config.TTLConfig{
			DemoDays: 1,
			FreeDays: 30,
			ProDays:  365,
		},
	}

	// Create JWT validator
	jwtValidator := auth.NewJWTValidator(cfg.JWT.Secret)

	// Create JSON validator
	validator, err := validation.NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Create repositories
	statsRepo := storage.NewRedisStatsRepository(redisClient)
	widgetRepo := storage.NewRedisWidgetRepository(redisClient, statsRepo)
	submissionRepo := storage.NewRedisSubmissionRepository(redisClient)

	// Create services
	ttlConfig := services.TTLConfig{
		DemoDays: cfg.TTL.DemoDays,
		FreeDays: cfg.TTL.FreeDays,
		ProDays:  cfg.TTL.ProDays,
	}
	widgetService := services.NewWidgetService(widgetRepo, submissionRepo, statsRepo, ttlConfig)
	exportService := services.NewExportService(submissionRepo, widgetRepo)

	// Create handler
	handler := NewWidgetHandler(widgetService, exportService, validator)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtValidator, false)

	// Create test user and token
	userID := "test-user-123"
	token := createTestJWTToken(cfg.JWT.Secret, userID)

	t.Cleanup(func() {
		universalClient.Close()
		mr.Close()
	})

	return &IntegrationTestEnvironment{
		Config:         cfg,
		JWTValidator:   jwtValidator,
		Validator:      validator,
		Redis:          mr,
		RedisClient:    redisClient,
		WidgetRepo:     widgetRepo,
		StatsRepo:      statsRepo,
		WidgetService:  widgetService,
		Handler:        handler,
		AuthMiddleware: authMiddleware,
		UserID:         userID,
		Token:          token,
	}
}

// createTestJWTToken creates a valid JWT token for testing
func createTestJWTToken(secret, userID string) string {
	// This is a simplified token creation for testing
	// In a real implementation, you'd use the proper JWT library
	return fmt.Sprintf("Bearer test-token-for-%s", userID)
}

// createTestWidget creates a widget for testing
func (env *IntegrationTestEnvironment) createTestWidget(id, name, widgetType string, isVisible bool, createdAt time.Time) *models.Widget {
	widget := &models.Widget{
		ID:        id,
		OwnerID:   env.UserID,
		Name:      name,
		Type:      widgetType,
		IsVisible: isVisible,
		Config:    map[string]interface{}{"test": "config"},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	err := env.WidgetRepo.Create(context.Background(), widget)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test widget: %v", err))
	}

	return widget
}

// makeAuthenticatedRequest creates an authenticated HTTP request
func (env *IntegrationTestEnvironment) makeAuthenticatedRequest(method, path string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	// Add authentication context (simulating middleware)
	user := &models.User{ID: env.UserID}
	ctx := auth.SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)

	return req
}

func TestGetWidgets_Integration_NoFilters(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets
	env.createTestWidget("widget-1", "Lead Form Widget", "lead-form", true, now.Add(-3*time.Hour))
	env.createTestWidget("widget-2", "Banner Widget", "banner", true, now.Add(-2*time.Hour))
	env.createTestWidget("widget-3", "Quiz Widget", "quiz", false, now.Add(-1*time.Hour))

	// Make request without filters
	req := env.makeAuthenticatedRequest("GET", "/api/v1/widgets", nil)
	w := httptest.NewRecorder()

	env.Handler.GetWidgets(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.WidgetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response.Meta == nil {
		t.Fatal("Meta information is missing")
	}

	if response.Meta.Total != 3 {
		t.Errorf("Expected total count of 3, got %d", response.Meta.Total)
	}

	if response.Meta.Page != 1 {
		t.Errorf("Expected page 1, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 20 {
		t.Errorf("Expected per_page 20, got %d", response.Meta.PerPage)
	}

	// Verify widgets are returned (response.Widgets is interface{})
	widgetsData, ok := response.Widgets.([]interface{})
	if !ok {
		t.Fatal("Widgets data is not in expected format")
	}

	if len(widgetsData) != 3 {
		t.Errorf("Expected 3 widgets, got %d", len(widgetsData))
	}

	t.Logf("Successfully retrieved %d widgets without filters", len(widgetsData))
}

func TestGetWidgets_Integration_TypeFilter(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets with different types
	env.createTestWidget("widget-1", "Lead Form 1", "lead-form", true, now.Add(-4*time.Hour))
	env.createTestWidget("widget-2", "Banner 1", "banner", true, now.Add(-3*time.Hour))
	env.createTestWidget("widget-3", "Lead Form 2", "lead-form", true, now.Add(-2*time.Hour))
	env.createTestWidget("widget-4", "Quiz 1", "quiz", true, now.Add(-1*time.Hour))

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
		description   string
	}{
		{
			name:          "single type filter",
			queryParams:   "type=lead-form",
			expectedCount: 2,
			description:   "should return only lead-form widgets",
		},
		{
			name:          "multiple type filter",
			queryParams:   "type=lead-form,banner",
			expectedCount: 3,
			description:   "should return lead-form and banner widgets",
		},
		{
			name:          "non-existent type",
			queryParams:   "type=non-existent",
			expectedCount: 4, // Invalid types are ignored, returns all widgets
			description:   "should return all widgets when invalid type is specified",
		},
		{
			name:          "mixed valid and invalid types",
			queryParams:   "type=banner,invalid-type",
			expectedCount: 1,
			description:   "should return only widgets matching valid types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/widgets"
			if tt.queryParams != "" {
				path += "?" + tt.queryParams
			}

			req := env.makeAuthenticatedRequest("GET", path, nil)
			w := httptest.NewRecorder()

			env.Handler.GetWidgets(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response models.WidgetsResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta.Total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, response.Meta.Total)
			}

			widgetsData, ok := response.Widgets.([]interface{})
			if !ok {
				t.Fatal("Widgets data is not in expected format")
			}

			if len(widgetsData) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(widgetsData))
			}

			t.Logf("Test '%s': %s - returned %d widgets", tt.name, tt.description, len(widgetsData))
		})
	}
}

func TestGetWidgets_Integration_VisibilityFilter(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets with different visibility
	env.createTestWidget("widget-1", "Visible Widget 1", "lead-form", true, now.Add(-3*time.Hour))
	env.createTestWidget("widget-2", "Hidden Widget 1", "banner", false, now.Add(-2*time.Hour))
	env.createTestWidget("widget-3", "Visible Widget 2", "quiz", true, now.Add(-1*time.Hour))

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
		description   string
	}{
		{
			name:          "visible widgets only",
			queryParams:   "isVisible=true",
			expectedCount: 2,
			description:   "should return only visible widgets",
		},
		{
			name:          "hidden widgets only",
			queryParams:   "isVisible=false",
			expectedCount: 1,
			description:   "should return only hidden widgets",
		},
		{
			name:          "invalid visibility value",
			queryParams:   "isVisible=invalid",
			expectedCount: 3, // Invalid values are ignored, returns all widgets
			description:   "should return all widgets when invalid visibility value is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/widgets?" + tt.queryParams

			req := env.makeAuthenticatedRequest("GET", path, nil)
			w := httptest.NewRecorder()

			env.Handler.GetWidgets(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response models.WidgetsResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta.Total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, response.Meta.Total)
			}

			widgetsData, ok := response.Widgets.([]interface{})
			if !ok {
				t.Fatal("Widgets data is not in expected format")
			}

			if len(widgetsData) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(widgetsData))
			}

			t.Logf("Test '%s': %s - returned %d widgets", tt.name, tt.description, len(widgetsData))
		})
	}
}

func TestGetWidgets_Integration_SearchFilter(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets with different names
	env.createTestWidget("widget-1", "Контактная форма", "lead-form", true, now.Add(-4*time.Hour))
	env.createTestWidget("widget-2", "Баннер рекламы", "banner", true, now.Add(-3*time.Hour))
	env.createTestWidget("widget-3", "Форма обратной связи", "lead-form", true, now.Add(-2*time.Hour))
	env.createTestWidget("widget-4", "Contact Form", "lead-form", true, now.Add(-1*time.Hour))

	tests := []struct {
		name          string
		searchTerm    string
		expectedCount int
		description   string
	}{
		{
			name:          "search for 'форма'",
			searchTerm:    "форма",
			expectedCount: 2,
			description:   "should return widgets containing 'форма' in name",
		},
		{
			name:          "search for 'contact' (case insensitive)",
			searchTerm:    "contact",
			expectedCount: 1,
			description:   "should return widgets containing 'contact' in name",
		},
		{
			name:          "search for 'баннер'",
			searchTerm:    "баннер",
			expectedCount: 1,
			description:   "should return widgets containing 'баннер' in name",
		},
		{
			name:          "search for non-existent term",
			searchTerm:    "несуществующий",
			expectedCount: 0,
			description:   "should return no widgets for non-existent search term",
		},
		{
			name:          "empty search term",
			searchTerm:    "",
			expectedCount: 4,
			description:   "should return all widgets for empty search term",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/widgets?search=%s", tt.searchTerm)

			req := env.makeAuthenticatedRequest("GET", path, nil)
			w := httptest.NewRecorder()

			env.Handler.GetWidgets(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response models.WidgetsResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta.Total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, response.Meta.Total)
			}

			widgetsData, ok := response.Widgets.([]interface{})
			if !ok {
				t.Fatal("Widgets data is not in expected format")
			}

			if len(widgetsData) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(widgetsData))
			}

			t.Logf("Test '%s': %s - returned %d widgets", tt.name, tt.description, len(widgetsData))
		})
	}
}

func TestGetWidgets_Integration_CombinedFilters(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets with various combinations
	env.createTestWidget("widget-1", "Контактная форма", "lead-form", true, now.Add(-5*time.Hour))
	env.createTestWidget("widget-2", "Скрытая форма", "lead-form", false, now.Add(-4*time.Hour))
	env.createTestWidget("widget-3", "Видимый баннер", "banner", true, now.Add(-3*time.Hour))
	env.createTestWidget("widget-4", "Скрытый баннер", "banner", false, now.Add(-2*time.Hour))
	env.createTestWidget("widget-5", "Форма опроса", "survey", true, now.Add(-1*time.Hour))

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
		description   string
	}{
		{
			name:          "type and visibility filter",
			queryParams:   "type=lead-form&isVisible=true",
			expectedCount: 1,
			description:   "should return only visible lead-form widgets",
		},
		{
			name:          "type and search filter",
			queryParams:   "type=lead-form,survey&search=форма",
			expectedCount: 3, // All widgets with "форма" in name that are lead-form or survey
			description:   "should return lead-form and survey widgets containing 'форма'",
		},
		{
			name:          "visibility and search filter",
			queryParams:   "isVisible=true&search=баннер",
			expectedCount: 1,
			description:   "should return visible widgets containing 'баннер'",
		},
		{
			name:          "all filters combined",
			queryParams:   "type=lead-form&isVisible=true&search=контакт",
			expectedCount: 1,
			description:   "should return visible lead-form widgets containing 'контакт'",
		},
		{
			name:          "filters with no results",
			queryParams:   "type=quiz&isVisible=true&search=форма",
			expectedCount: 0,
			description:   "should return no widgets when filters don't match any widgets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/widgets?" + tt.queryParams

			req := env.makeAuthenticatedRequest("GET", path, nil)
			w := httptest.NewRecorder()

			env.Handler.GetWidgets(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response models.WidgetsResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta.Total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, response.Meta.Total)
			}

			widgetsData, ok := response.Widgets.([]interface{})
			if !ok {
				t.Fatal("Widgets data is not in expected format")
			}

			if len(widgetsData) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(widgetsData))
			}

			t.Logf("Test '%s': %s - returned %d widgets", tt.name, tt.description, len(widgetsData))
		})
	}
}

func TestGetWidgets_Integration_Pagination(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create 10 test widgets
	for i := 1; i <= 10; i++ {
		env.createTestWidget(
			fmt.Sprintf("widget-%d", i),
			fmt.Sprintf("Test Widget %d", i),
			"lead-form",
			true,
			now.Add(-time.Duration(10-i)*time.Hour), // Newer widgets have higher numbers
		)
	}

	tests := []struct {
		name          string
		queryParams   string
		expectedCount int
		expectedTotal int
		expectedPage  int
		description   string
	}{
		{
			name:          "first page",
			queryParams:   "page=1&per_page=3&type=lead-form",
			expectedCount: 3,
			expectedTotal: 10,
			expectedPage:  1,
			description:   "should return first 3 widgets",
		},
		{
			name:          "second page",
			queryParams:   "page=2&per_page=3&type=lead-form",
			expectedCount: 3,
			expectedTotal: 10,
			expectedPage:  2,
			description:   "should return next 3 widgets",
		},
		{
			name:          "last page",
			queryParams:   "page=4&per_page=3&type=lead-form",
			expectedCount: 1,
			expectedTotal: 10,
			expectedPage:  4,
			description:   "should return remaining widget",
		},
		{
			name:          "page beyond results",
			queryParams:   "page=5&per_page=3&type=lead-form",
			expectedCount: 0,
			expectedTotal: 10,
			expectedPage:  5,
			description:   "should return no widgets for page beyond results",
		},
		{
			name:          "large page size",
			queryParams:   "page=1&per_page=20&type=lead-form",
			expectedCount: 10,
			expectedTotal: 10,
			expectedPage:  1,
			description:   "should return all widgets when page size is larger than total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/widgets?" + tt.queryParams

			req := env.makeAuthenticatedRequest("GET", path, nil)
			w := httptest.NewRecorder()

			env.Handler.GetWidgets(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response models.WidgetsResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta.Total != tt.expectedTotal {
				t.Errorf("Expected total count of %d, got %d", tt.expectedTotal, response.Meta.Total)
			}

			if response.Meta.Page != tt.expectedPage {
				t.Errorf("Expected page %d, got %d", tt.expectedPage, response.Meta.Page)
			}

			widgetsData, ok := response.Widgets.([]interface{})
			if !ok {
				t.Fatal("Widgets data is not in expected format")
			}

			if len(widgetsData) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(widgetsData))
			}

			// Verify HasMore flag - should be true if we got a full page and there might be more
			perPage := 3
			if tt.queryParams == "page=1&per_page=20&type=lead-form" {
				perPage = 20
			}
			expectedHasMore := len(widgetsData) == perPage && (tt.expectedPage*perPage) < tt.expectedTotal
			if response.Meta.HasMore != expectedHasMore {
				t.Errorf("Expected HasMore %t, got %t", expectedHasMore, response.Meta.HasMore)
			}

			t.Logf("Test '%s': %s - page %d returned %d/%d widgets", tt.name, tt.description, tt.expectedPage, len(widgetsData), tt.expectedTotal)
		})
	}
}

func TestGetWidgets_Integration_ErrorHandling(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)

	tests := []struct {
		name           string
		setupAuth      bool
		expectedStatus int
		description    string
	}{
		{
			name:           "unauthenticated request",
			setupAuth:      false,
			expectedStatus: http.StatusUnauthorized,
			description:    "should return 401 for unauthenticated requests",
		},
		{
			name:           "authenticated request",
			setupAuth:      true,
			expectedStatus: http.StatusOK,
			description:    "should return 200 for authenticated requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.setupAuth {
				req = env.makeAuthenticatedRequest("GET", "/api/v1/widgets", nil)
			} else {
				req = httptest.NewRequest("GET", "/api/v1/widgets", nil)
			}

			w := httptest.NewRecorder()
			env.Handler.GetWidgets(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			t.Logf("Test '%s': %s - returned status %d", tt.name, tt.description, w.Code)
		})
	}
}

func TestGetWidgets_Integration_BackwardCompatibility(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets
	env.createTestWidget("widget-1", "Test Widget 1", "lead-form", true, now.Add(-2*time.Hour))
	env.createTestWidget("widget-2", "Test Widget 2", "banner", true, now.Add(-1*time.Hour))

	// Test that existing API calls without filters still work
	req := env.makeAuthenticatedRequest("GET", "/api/v1/widgets", nil)
	w := httptest.NewRecorder()

	env.Handler.GetWidgets(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.WidgetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure is unchanged
	if response.Meta == nil {
		t.Fatal("Meta information is missing")
	}

	if response.Meta.Total != 2 {
		t.Errorf("Expected total count of 2, got %d", response.Meta.Total)
	}

	// Verify default pagination values
	if response.Meta.Page != 1 {
		t.Errorf("Expected default page 1, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 20 {
		t.Errorf("Expected default per_page 20, got %d", response.Meta.PerPage)
	}

	// Test with existing pagination parameters
	req2 := env.makeAuthenticatedRequest("GET", "/api/v1/widgets?page=1&per_page=10", nil)
	w2 := httptest.NewRecorder()

	env.Handler.GetWidgets(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w2.Code)
	}

	var response2 models.WidgetsResponse
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response2.Meta.PerPage != 10 {
		t.Errorf("Expected per_page 10, got %d", response2.Meta.PerPage)
	}

	t.Log("Backward compatibility verified - existing API calls work without changes")
}

func TestGetWidgets_Integration_MetaInformation(t *testing.T) {
	env := setupIntegrationTestEnvironment(t)
	now := time.Now()

	// Create test widgets
	for i := 1; i <= 5; i++ {
		env.createTestWidget(
			fmt.Sprintf("widget-%d", i),
			fmt.Sprintf("Test Widget %d", i),
			"lead-form",
			i%2 == 1, // Alternate visibility
			now.Add(-time.Duration(i)*time.Hour),
		)
	}

	tests := []struct {
		name         string
		queryParams  string
		expectedMeta models.Meta
		description  string
	}{
		{
			name:        "no filters - all widgets",
			queryParams: "",
			expectedMeta: models.Meta{
				Page:    1,
				PerPage: 20,
				Total:   5,
				HasMore: false,
			},
			description: "should return correct meta for all widgets",
		},
		{
			name:        "visibility filter - visible only",
			queryParams: "isVisible=true",
			expectedMeta: models.Meta{
				Page:    1,
				PerPage: 20,
				Total:   3, // 3 visible widgets
				HasMore: false,
			},
			description: "should return correct meta for filtered results",
		},
		{
			name:        "pagination with filters",
			queryParams: "type=lead-form&page=1&per_page=2",
			expectedMeta: models.Meta{
				Page:    1,
				PerPage: 2,
				Total:   5,    // All widgets are lead-form
				HasMore: true, // More pages available
			},
			description: "should return correct meta for paginated filtered results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/widgets"
			if tt.queryParams != "" {
				path += "?" + tt.queryParams
			}

			req := env.makeAuthenticatedRequest("GET", path, nil)
			w := httptest.NewRecorder()

			env.Handler.GetWidgets(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
			}

			var response models.WidgetsResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta == nil {
				t.Fatal("Meta information is missing")
			}

			meta := response.Meta
			if meta.Page != tt.expectedMeta.Page {
				t.Errorf("Expected page %d, got %d", tt.expectedMeta.Page, meta.Page)
			}

			if meta.PerPage != tt.expectedMeta.PerPage {
				t.Errorf("Expected per_page %d, got %d", tt.expectedMeta.PerPage, meta.PerPage)
			}

			if meta.Total != tt.expectedMeta.Total {
				t.Errorf("Expected total %d, got %d", tt.expectedMeta.Total, meta.Total)
			}

			if meta.HasMore != tt.expectedMeta.HasMore {
				t.Errorf("Expected has_more %t, got %t", tt.expectedMeta.HasMore, meta.HasMore)
			}

			t.Logf("Test '%s': %s - meta verified", tt.name, tt.description)
		})
	}
}
