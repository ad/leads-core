package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/models"
)

// BenchmarkWidgetRepository_GetByUserIDWithFilters benchmarks the repository filtering methods
func BenchmarkWidgetRepository_GetByUserIDWithFilters(b *testing.B) {
	// Test with different data sizes
	testSizes := []int{10, 100, 1000}

	for _, size := range testSizes {
		b.Run(fmt.Sprintf("NoFilters_%d_widgets", size), func(b *testing.B) {
			benchmarkRepositoryNoFilters(b, size)
		})

		b.Run(fmt.Sprintf("TypeFilter_%d_widgets", size), func(b *testing.B) {
			benchmarkRepositoryTypeFilter(b, size)
		})

		b.Run(fmt.Sprintf("VisibilityFilter_%d_widgets", size), func(b *testing.B) {
			benchmarkRepositoryVisibilityFilter(b, size)
		})

		b.Run(fmt.Sprintf("SearchFilter_%d_widgets", size), func(b *testing.B) {
			benchmarkRepositorySearchFilter(b, size)
		})

		b.Run(fmt.Sprintf("CombinedFilters_%d_widgets", size), func(b *testing.B) {
			benchmarkRepositoryCombinedFilters(b, size)
		})

		b.Run(fmt.Sprintf("MultipleTypes_%d_widgets", size), func(b *testing.B) {
			benchmarkRepositoryMultipleTypes(b, size)
		})
	}
}

// benchmarkRepositoryNoFilters benchmarks baseline repository performance
func benchmarkRepositoryNoFilters(b *testing.B, widgetCount int) {
	repo, userID := setupBenchmarkRepository(b, widgetCount)

	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 20,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := repo.GetByUserIDWithFilters(context.Background(), userID, opts)
		if err != nil {
			b.Fatalf("GetByUserIDWithFilters failed: %v", err)
		}
	}
}

// benchmarkRepositoryTypeFilter benchmarks type filtering performance
func benchmarkRepositoryTypeFilter(b *testing.B, widgetCount int) {
	repo, userID := setupBenchmarkRepository(b, widgetCount)

	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 20,
		Filters: &models.FilterOptions{
			Types: []string{"lead-form"},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := repo.GetByUserIDWithFilters(context.Background(), userID, opts)
		if err != nil {
			b.Fatalf("GetByUserIDWithFilters failed: %v", err)
		}
	}
}

// benchmarkRepositoryVisibilityFilter benchmarks visibility filtering performance
func benchmarkRepositoryVisibilityFilter(b *testing.B, widgetCount int) {
	repo, userID := setupBenchmarkRepository(b, widgetCount)

	visible := true
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 20,
		Filters: &models.FilterOptions{
			IsVisible: &visible,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := repo.GetByUserIDWithFilters(context.Background(), userID, opts)
		if err != nil {
			b.Fatalf("GetByUserIDWithFilters failed: %v", err)
		}
	}
}

// benchmarkRepositorySearchFilter benchmarks search filtering performance
func benchmarkRepositorySearchFilter(b *testing.B, widgetCount int) {
	repo, userID := setupBenchmarkRepository(b, widgetCount)

	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 20,
		Filters: &models.FilterOptions{
			Search: "test",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := repo.GetByUserIDWithFilters(context.Background(), userID, opts)
		if err != nil {
			b.Fatalf("GetByUserIDWithFilters failed: %v", err)
		}
	}
}

// benchmarkRepositoryCombinedFilters benchmarks combined filtering performance
func benchmarkRepositoryCombinedFilters(b *testing.B, widgetCount int) {
	repo, userID := setupBenchmarkRepository(b, widgetCount)

	visible := true
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 20,
		Filters: &models.FilterOptions{
			Types:     []string{"lead-form"},
			IsVisible: &visible,
			Search:    "test",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := repo.GetByUserIDWithFilters(context.Background(), userID, opts)
		if err != nil {
			b.Fatalf("GetByUserIDWithFilters failed: %v", err)
		}
	}
}

// benchmarkRepositoryMultipleTypes benchmarks multiple type filtering performance
func benchmarkRepositoryMultipleTypes(b *testing.B, widgetCount int) {
	repo, userID := setupBenchmarkRepository(b, widgetCount)

	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 20,
		Filters: &models.FilterOptions{
			Types: []string{"lead-form", "banner", "quiz"},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := repo.GetByUserIDWithFilters(context.Background(), userID, opts)
		if err != nil {
			b.Fatalf("GetByUserIDWithFilters failed: %v", err)
		}
	}
}

// setupBenchmarkRepository creates a mock repository with test data
func setupBenchmarkRepository(b *testing.B, widgetCount int) (WidgetRepository, string) {
	// Create mock repository
	repo := &MockBenchmarkWidgetRepository{
		widgets:     make(map[string]*models.Widget),
		userWidgets: make(map[string][]string),
		typeIndex:   make(map[string][]string),
		statusIndex: make(map[bool][]string),
	}

	userID := "benchmark-user-123"
	widgetTypes := []string{"lead-form", "banner", "quiz", "survey", "popup"}

	// Create test widgets
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

		repo.widgets[widgetID] = widget
		repo.userWidgets[userID] = append(repo.userWidgets[userID], widgetID)
		repo.typeIndex[widgetType] = append(repo.typeIndex[widgetType], widgetID)
		repo.statusIndex[isVisible] = append(repo.statusIndex[isVisible], widgetID)
	}

	return repo, userID
}

// MockBenchmarkWidgetRepository simulates Redis operations for benchmarking
type MockBenchmarkWidgetRepository struct {
	widgets     map[string]*models.Widget
	userWidgets map[string][]string
	typeIndex   map[string][]string
	statusIndex map[bool][]string
}

func (m *MockBenchmarkWidgetRepository) Create(ctx context.Context, widget *models.Widget) error {
	return nil
}

func (m *MockBenchmarkWidgetRepository) GetByID(ctx context.Context, id string) (*models.Widget, error) {
	if widget, exists := m.widgets[id]; exists {
		return widget, nil
	}
	return nil, fmt.Errorf("widget not found")
}

func (m *MockBenchmarkWidgetRepository) GetByUserID(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	return m.GetByUserIDWithFilters(ctx, userID, opts)
}

func (m *MockBenchmarkWidgetRepository) RebuildIndexes(ctx context.Context) error {
	// Mock implementation - no-op for benchmarks
	return nil
}

func (m *MockBenchmarkWidgetRepository) GetTypeStats(ctx context.Context, userID string) ([]*models.TypeStats, error) {
	// Simple mock implementation for benchmarks
	typeCounts := make(map[string]int)
	for _, widget := range m.widgets {
		typeCounts[widget.Type]++
	}

	var stats []*models.TypeStats
	for widgetType, count := range typeCounts {
		if count > 0 {
			stats = append(stats, &models.TypeStats{
				Type:  widgetType,
				Count: count,
			})
		}
	}
	return stats, nil
}

func (m *MockBenchmarkWidgetRepository) GetByUserIDWithFilters(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	// Simulate Redis operations

	// Step 1: Get user widgets (simulates ZREVRANGE)
	userWidgetIDs, exists := m.userWidgets[userID]
	if !exists {
		return []*models.Widget{}, 0, nil
	}

	var filteredIDs []string

	// Step 2: Apply filters if present
	if opts.Filters != nil && opts.Filters.HasFilters() {
		filteredIDs = m.applyFiltersSimulatingRedis(userWidgetIDs, opts.Filters)
	} else {
		filteredIDs = userWidgetIDs
	}

	total := len(filteredIDs)

	// Step 3: Apply pagination
	start := (opts.Page - 1) * opts.PerPage
	end := start + opts.PerPage

	if start >= total {
		return []*models.Widget{}, total, nil
	}

	if end > total {
		end = total
	}

	paginatedIDs := filteredIDs[start:end]

	// Step 4: Load widgets (simulates multiple HGETALL operations)
	widgets := make([]*models.Widget, 0, len(paginatedIDs))
	for _, widgetID := range paginatedIDs {
		if widget, exists := m.widgets[widgetID]; exists {
			// Simulate stats loading
			widget.Stats = &models.WidgetStats{
				WidgetID: widgetID,
				Views:    100,
				Submits:  10,
				Closes:   5,
			}
			widgets = append(widgets, widget)
		}
	}

	return widgets, total, nil
}

func (m *MockBenchmarkWidgetRepository) applyFiltersSimulatingRedis(userWidgetIDs []string, filters *models.FilterOptions) []string {
	// Convert user widgets to set for intersection (simulates creating temp SET)
	userWidgetSet := make(map[string]bool)
	for _, id := range userWidgetIDs {
		userWidgetSet[id] = true
	}

	var candidateIDs []string

	// Apply type filter (simulates SINTER with type sets)
	if filters.HasTypeFilter() {
		if len(filters.Types) == 1 {
			// Single type filter (simulates direct SINTER)
			typeWidgets := m.typeIndex[filters.Types[0]]
			for _, widgetID := range typeWidgets {
				if userWidgetSet[widgetID] {
					candidateIDs = append(candidateIDs, widgetID)
				}
			}
		} else {
			// Multiple types (simulates SUNION then SINTER)
			typeUnion := make(map[string]bool)
			for _, widgetType := range filters.Types {
				for _, widgetID := range m.typeIndex[widgetType] {
					typeUnion[widgetID] = true
				}
			}

			for widgetID := range typeUnion {
				if userWidgetSet[widgetID] {
					candidateIDs = append(candidateIDs, widgetID)
				}
			}
		}
	} else {
		candidateIDs = userWidgetIDs
	}

	// Apply visibility filter (simulates SINTER with status set)
	if filters.HasVisibilityFilter() {
		statusWidgets := m.statusIndex[*filters.IsVisible]
		statusSet := make(map[string]bool)
		for _, widgetID := range statusWidgets {
			statusSet[widgetID] = true
		}

		var visibilityFiltered []string
		for _, widgetID := range candidateIDs {
			if statusSet[widgetID] {
				visibilityFiltered = append(visibilityFiltered, widgetID)
			}
		}
		candidateIDs = visibilityFiltered
	}

	// Apply search filter (simulates loading widgets and checking names)
	if filters.HasSearchFilter() {
		var searchFiltered []string
		for _, widgetID := range candidateIDs {
			if widget, exists := m.widgets[widgetID]; exists {
				if containsIgnoreCase(widget.Name, filters.Search) {
					searchFiltered = append(searchFiltered, widgetID)
				}
			}
		}
		candidateIDs = searchFiltered
	}

	return candidateIDs
}

func (m *MockBenchmarkWidgetRepository) Update(ctx context.Context, widget *models.Widget) error {
	return nil
}

func (m *MockBenchmarkWidgetRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockBenchmarkWidgetRepository) GetWidgetsByType(ctx context.Context, widgetType string, opts models.PaginationOptions) ([]*models.Widget, error) {
	return nil, nil
}

func (m *MockBenchmarkWidgetRepository) GetWidgetsByStatus(ctx context.Context, enabled bool, opts models.PaginationOptions) ([]*models.Widget, error) {
	return nil, nil
}

// BenchmarkFilterOptions_HasFilters benchmarks filter option methods
func BenchmarkFilterOptions_HasFilters(b *testing.B) {
	filters := &models.FilterOptions{
		Types:     []string{"lead-form", "banner"},
		IsVisible: &[]bool{true}[0],
		Search:    "test search",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = filters.HasFilters()
		_ = filters.HasTypeFilter()
		_ = filters.HasVisibilityFilter()
		_ = filters.HasSearchFilter()
	}
}

// BenchmarkValidateFilterOptions benchmarks filter validation
func BenchmarkValidateFilterOptions(b *testing.B) {
	filters := &models.FilterOptions{
		Types:     []string{"lead-form", "invalid-type", "banner", ""},
		IsVisible: &[]bool{true}[0],
		Search:    "  test search  ",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = models.ValidateFilterOptions(filters)
	}
}

// Helper function for case-insensitive search (simplified for benchmarking)
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	// Simplified case-insensitive check for benchmarking
	sLower := strings.ToLower(s)
	substrLower := strings.ToLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}

	return false
}
