package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/auth"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/validation"
)

// BenchmarkWidgetHandler_GetWidgets benchmarks the GetWidgets handler with various scenarios
func BenchmarkWidgetHandler_GetWidgets(b *testing.B) {
	// Setup test data sizes
	testSizes := []int{10, 100, 1000}

	for _, size := range testSizes {
		b.Run(fmt.Sprintf("NoFilters_%d_widgets", size), func(b *testing.B) {
			benchmarkGetWidgetsNoFilters(b, size)
		})

		b.Run(fmt.Sprintf("TypeFilter_%d_widgets", size), func(b *testing.B) {
			benchmarkGetWidgetsTypeFilter(b, size)
		})

		b.Run(fmt.Sprintf("VisibilityFilter_%d_widgets", size), func(b *testing.B) {
			benchmarkGetWidgetsVisibilityFilter(b, size)
		})

		b.Run(fmt.Sprintf("SearchFilter_%d_widgets", size), func(b *testing.B) {
			benchmarkGetWidgetsSearchFilter(b, size)
		})

		b.Run(fmt.Sprintf("CombinedFilters_%d_widgets", size), func(b *testing.B) {
			benchmarkGetWidgetsCombinedFilters(b, size)
		})
	}
}

// benchmarkGetWidgetsNoFilters benchmarks baseline performance without filters
func benchmarkGetWidgetsNoFilters(b *testing.B, widgetCount int) {
	handler, userID := setupBenchmarkHandler(b, widgetCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createBenchmarkRequest(userID, map[string]string{
			"page":     "1",
			"per_page": "20",
		})

		w := httptest.NewRecorder()
		handler.GetWidgets(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// benchmarkGetWidgetsTypeFilter benchmarks performance with type filter
func benchmarkGetWidgetsTypeFilter(b *testing.B, widgetCount int) {
	handler, userID := setupBenchmarkHandler(b, widgetCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createBenchmarkRequest(userID, map[string]string{
			"page":     "1",
			"per_page": "20",
			"type":     "lead-form",
		})

		w := httptest.NewRecorder()
		handler.GetWidgets(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// benchmarkGetWidgetsVisibilityFilter benchmarks performance with visibility filter
func benchmarkGetWidgetsVisibilityFilter(b *testing.B, widgetCount int) {
	handler, userID := setupBenchmarkHandler(b, widgetCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createBenchmarkRequest(userID, map[string]string{
			"page":      "1",
			"per_page":  "20",
			"isVisible": "true",
		})

		w := httptest.NewRecorder()
		handler.GetWidgets(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// benchmarkGetWidgetsSearchFilter benchmarks performance with search filter
func benchmarkGetWidgetsSearchFilter(b *testing.B, widgetCount int) {
	handler, userID := setupBenchmarkHandler(b, widgetCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createBenchmarkRequest(userID, map[string]string{
			"page":     "1",
			"per_page": "20",
			"search":   "test",
		})

		w := httptest.NewRecorder()
		handler.GetWidgets(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// benchmarkGetWidgetsCombinedFilters benchmarks performance with multiple filters
func benchmarkGetWidgetsCombinedFilters(b *testing.B, widgetCount int) {
	handler, userID := setupBenchmarkHandler(b, widgetCount)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createBenchmarkRequest(userID, map[string]string{
			"page":      "1",
			"per_page":  "20",
			"type":      "lead-form,banner",
			"isVisible": "true",
			"search":    "test",
		})

		w := httptest.NewRecorder()
		handler.GetWidgets(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// setupBenchmarkHandler creates a handler with test data for benchmarking
func setupBenchmarkHandler(b *testing.B, widgetCount int) (*WidgetHandler, string) {
	// Create mock repositories
	mockWidgetRepo := &MockWidgetRepository{
		widgets: make(map[string]*models.Widget),
	}
	mockSubmissionRepo := &MockSubmissionRepository{}
	mockStatsRepo := &MockStatsRepository{
		stats: make(map[string]*models.WidgetStats),
	}

	// Create test user
	userID := "benchmark-user-123"

	// Create test widgets with varied data
	widgetTypes := []string{"lead-form", "banner", "quiz", "survey", "popup"}

	for i := 0; i < widgetCount; i++ {
		widgetID := fmt.Sprintf("widget-%d", i)
		widgetType := widgetTypes[i%len(widgetTypes)]
		isVisible := i%3 != 0 // ~67% visible

		widget := &models.Widget{
			ID:        widgetID,
			OwnerID:   userID,
			Type:      widgetType,
			Name:      fmt.Sprintf("Test Widget %d", i),
			IsVisible: isVisible,
			Config:    map[string]interface{}{"test": "config"},
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
			UpdatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}

		mockWidgetRepo.widgets[widgetID] = widget

		// Create stats for each widget
		mockStatsRepo.stats[widgetID] = &models.WidgetStats{
			WidgetID: widgetID,
			Views:    int64(i * 10),
			Submits:  int64(i * 2),
			Closes:   int64(i * 5),
		}
	}

	// Setup mock repository behavior
	mockWidgetRepo.setupBenchmarkBehavior(userID, widgetCount)

	// Create service
	widgetService := services.NewWidgetService(
		mockWidgetRepo,
		mockSubmissionRepo,
		mockStatsRepo,
		services.TTLConfig{DemoDays: 7, FreeDays: 30, ProDays: 365},
	)

	// Create handler
	validator := &validation.SchemaValidator{}
	exportService := &services.ExportService{}
	handler := NewWidgetHandler(widgetService, exportService, validator)

	return handler, userID
}

// createBenchmarkRequest creates an HTTP request for benchmarking
func createBenchmarkRequest(userID string, params map[string]string) *http.Request {
	// Create URL with query parameters
	u := &url.URL{Path: "/api/v1/widgets"}
	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	req := httptest.NewRequest("GET", u.String(), nil)

	// Add user to context
	user := &models.User{ID: userID}
	ctx := auth.SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)

	return req
}

// MockWidgetRepository for benchmarking
type MockWidgetRepository struct {
	widgets map[string]*models.Widget
}

func (m *MockWidgetRepository) Create(ctx context.Context, widget *models.Widget) error {
	m.widgets[widget.ID] = widget
	return nil
}

func (m *MockWidgetRepository) GetByID(ctx context.Context, id string) (*models.Widget, error) {
	if widget, exists := m.widgets[id]; exists {
		return widget, nil
	}
	return nil, fmt.Errorf("widget not found")
}

func (m *MockWidgetRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	return m.GetByUserIDWithFilters(ctx, userID, opts)
}

func (m *MockWidgetRepository) GetByUserIDWithFilters(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	// Collect user's widgets
	var userWidgets []*models.Widget
	for _, widget := range m.widgets {
		if widget.OwnerID == userID {
			userWidgets = append(userWidgets, widget)
		}
	}

	// Apply filters if present
	if opts.Filters != nil && opts.Filters.HasFilters() {
		userWidgets = m.applyFilters(userWidgets, opts.Filters)
	}

	total := len(userWidgets)

	// Apply pagination
	start := (opts.Page - 1) * opts.PerPage
	end := start + opts.PerPage

	if start >= total {
		return []*models.Widget{}, total, nil
	}

	if end > total {
		end = total
	}

	return userWidgets[start:end], total, nil
}

func (m *MockWidgetRepository) applyFilters(widgets []*models.Widget, filters *models.FilterOptions) []*models.Widget {
	var filtered []*models.Widget

	for _, widget := range widgets {
		// Apply type filter
		if filters.HasTypeFilter() {
			typeMatch := false
			for _, filterType := range filters.Types {
				if widget.Type == filterType {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				continue
			}
		}

		// Apply visibility filter
		if filters.HasVisibilityFilter() && widget.IsVisible != *filters.IsVisible {
			continue
		}

		// Apply search filter
		if filters.HasSearchFilter() {
			if !containsIgnoreCase(widget.Name, filters.Search) {
				continue
			}
		}

		filtered = append(filtered, widget)
	}

	return filtered
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	// Simple case-insensitive check
	sLower := strings.ToLower(s)
	substrLower := strings.ToLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}

	return false
}

func (m *MockWidgetRepository) setupBenchmarkBehavior(userID string, widgetCount int) {
	// This method can be used to setup specific behavior for benchmarks
	// Currently, the behavior is handled in the methods above
}

func (m *MockWidgetRepository) Update(ctx context.Context, widget *models.Widget) error {
	m.widgets[widget.ID] = widget
	return nil
}

func (m *MockWidgetRepository) Delete(ctx context.Context, id string) error {
	delete(m.widgets, id)
	return nil
}

func (m *MockWidgetRepository) GetWidgetsByType(ctx context.Context, widgetType string, opts models.PaginationOptions) ([]*models.Widget, error) {
	var widgets []*models.Widget
	for _, widget := range m.widgets {
		if widget.Type == widgetType {
			widgets = append(widgets, widget)
		}
	}
	return widgets, nil
}

func (m *MockWidgetRepository) GetWidgetsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Widget, error) {
	var widgets []*models.Widget
	for _, widget := range m.widgets {
		if widget.IsVisible == enabled {
			widgets = append(widgets, widget)
		}
	}
	return widgets, nil
}

// MockSubmissionRepository for benchmarking
type MockSubmissionRepository struct{}

func (m *MockSubmissionRepository) Create(ctx context.Context, submission *models.Submission) error {
	return nil
}

func (m *MockSubmissionRepository) GetByID(ctx context.Context, widgetID, submissionID string) (*models.Submission, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockSubmissionRepository) GetByWidgetID(ctx context.Context, widgetID string, opts models.PaginationOptions) ([]*models.Submission, int, error) {
	return []*models.Submission{}, 0, nil
}

func (m *MockSubmissionRepository) UpdateTTL(ctx context.Context, userID string, ttl time.Duration) error {
	return nil
}

func (m *MockSubmissionRepository) UpdateWidgetSubmissionsTTL(ctx context.Context, widgetID string, ttlDays int) error {
	return nil
}

func (m *MockSubmissionRepository) CleanupExpired(ctx context.Context) (int, error) {
	return 0, nil
}

// MockStatsRepository for benchmarking
type MockStatsRepository struct {
	stats map[string]*models.WidgetStats
}

func (m *MockStatsRepository) GetWidgetStats(ctx context.Context, widgetID string) (*models.WidgetStats, error) {
	if stats, exists := m.stats[widgetID]; exists {
		return stats, nil
	}
	return &models.WidgetStats{
		WidgetID: widgetID,
		Views:    0,
		Submits:  0,
		Closes:   0,
	}, nil
}

func (m *MockStatsRepository) IncrementViews(ctx context.Context, widgetID string) error {
	if stats, exists := m.stats[widgetID]; exists {
		stats.Views++
	}
	return nil
}

func (m *MockStatsRepository) IncrementSubmits(ctx context.Context, widgetID string) error {
	if stats, exists := m.stats[widgetID]; exists {
		stats.Submits++
	}
	return nil
}

func (m *MockStatsRepository) IncrementCloses(ctx context.Context, widgetID string) error {
	if stats, exists := m.stats[widgetID]; exists {
		stats.Closes++
	}
	return nil
}

func (m *MockStatsRepository) GetDailyViews(ctx context.Context, widgetID, date string) (int64, error) {
	return 0, nil
}
