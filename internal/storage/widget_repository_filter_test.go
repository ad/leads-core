package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupTestRedisForFiltering creates a miniredis instance with proper Redis client for filtering tests
func setupTestRedisForFiltering(t *testing.T) (*RedisClient, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	redisClient := &RedisClient{client: client}

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return redisClient, cleanup
}

// createTestWidget creates a widget for testing
func createTestWidget(id, ownerID, name, widgetType string, isVisible bool, createdAt time.Time) *models.Widget {
	return &models.Widget{
		ID:        id,
		OwnerID:   ownerID,
		Name:      name,
		Type:      widgetType,
		IsVisible: isVisible,
		Config:    map[string]interface{}{"test": "config"},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_NoFilters(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	// Create mock stats repository
	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create test widgets
	widgets := []*models.Widget{
		createTestWidget("widget-1", userID, "Lead Form Widget", "lead-form", true, now.Add(-3*time.Hour)),
		createTestWidget("widget-2", userID, "Banner Widget", "banner", true, now.Add(-2*time.Hour)),
		createTestWidget("widget-3", userID, "Quiz Widget", "quiz", false, now.Add(-1*time.Hour)),
		createTestWidget("widget-4", "other-user", "Other User Widget", "lead-form", true, now),
	}

	// Create widgets in repository
	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	// Test without filters (should behave like GetByUserID)
	opts := models.PaginationOptions{
		Page:    1,
		PerPage: 10,
		Filters: nil, // No filters
	}

	result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
	if err != nil {
		t.Fatalf("GetByUserIDWithFilters failed: %v", err)
	}

	// Should return all widgets for the user (3 widgets)
	if len(result) != 3 {
		t.Errorf("Expected 3 widgets, got %d", len(result))
	}

	if total != 3 {
		t.Errorf("Expected total count of 3, got %d", total)
	}

	// Verify widgets are sorted by creation time (newest first)
	if len(result) >= 2 {
		if result[0].CreatedAt.Before(result[1].CreatedAt) {
			t.Error("Widgets should be sorted by creation time (newest first)")
		}
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_TypeFilter(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create test widgets with different types
	widgets := []*models.Widget{
		createTestWidget("widget-1", userID, "Lead Form 1", "lead-form", true, now.Add(-4*time.Hour)),
		createTestWidget("widget-2", userID, "Banner 1", "banner", true, now.Add(-3*time.Hour)),
		createTestWidget("widget-3", userID, "Lead Form 2", "lead-form", true, now.Add(-2*time.Hour)),
		createTestWidget("widget-4", userID, "Quiz 1", "quiz", true, now.Add(-1*time.Hour)),
		createTestWidget("widget-5", userID, "Survey 1", "survey", true, now),
	}

	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	tests := []struct {
		name            string
		filterTypes     []string
		expectedCount   int
		expectedWidgets []string
	}{
		{
			name:            "single type filter - lead-form",
			filterTypes:     []string{"lead-form"},
			expectedCount:   2,
			expectedWidgets: []string{"widget-3", "widget-1"}, // Sorted by creation time (newest first)
		},
		{
			name:            "single type filter - banner",
			filterTypes:     []string{"banner"},
			expectedCount:   1,
			expectedWidgets: []string{"widget-2"},
		},
		{
			name:            "multiple type filter",
			filterTypes:     []string{"lead-form", "quiz"},
			expectedCount:   3,
			expectedWidgets: []string{"widget-4", "widget-3", "widget-1"}, // Sorted by creation time
		},
		{
			name:            "non-existent type (should return all widgets as invalid types are ignored)",
			filterTypes:     []string{"non-existent"},
			expectedCount:   5,                                                                    // All widgets returned when no valid filters remain
			expectedWidgets: []string{"widget-5", "widget-4", "widget-3", "widget-2", "widget-1"}, // All widgets sorted by creation time
		},
		{
			name:            "mixed valid and invalid types",
			filterTypes:     []string{"banner", "invalid-type", "survey"},
			expectedCount:   2,
			expectedWidgets: []string{"widget-5", "widget-2"}, // Only valid types
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.PaginationOptions{
				Page:    1,
				PerPage: 10,
				Filters: &models.FilterOptions{
					Types: tt.filterTypes,
				},
			}

			result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
			if err != nil {
				t.Fatalf("GetByUserIDWithFilters failed: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(result))
			}

			if total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, total)
			}

			// Verify correct widgets are returned
			for i, expectedID := range tt.expectedWidgets {
				if i >= len(result) {
					t.Errorf("Missing expected widget %s at index %d", expectedID, i)
					continue
				}
				if result[i].ID != expectedID {
					t.Errorf("Expected widget %s at index %d, got %s", expectedID, i, result[i].ID)
				}
			}
		})
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_VisibilityFilter(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create test widgets with different visibility
	widgets := []*models.Widget{
		createTestWidget("widget-1", userID, "Visible Widget 1", "lead-form", true, now.Add(-3*time.Hour)),
		createTestWidget("widget-2", userID, "Hidden Widget 1", "banner", false, now.Add(-2*time.Hour)),
		createTestWidget("widget-3", userID, "Visible Widget 2", "quiz", true, now.Add(-1*time.Hour)),
		createTestWidget("widget-4", userID, "Hidden Widget 2", "survey", false, now),
	}

	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	tests := []struct {
		name            string
		isVisible       *bool
		expectedCount   int
		expectedWidgets []string
	}{
		{
			name:            "visible widgets only",
			isVisible:       boolPtr(true),
			expectedCount:   2,
			expectedWidgets: []string{"widget-3", "widget-1"}, // Sorted by creation time (newest first)
		},
		{
			name:            "hidden widgets only",
			isVisible:       boolPtr(false),
			expectedCount:   2,
			expectedWidgets: []string{"widget-4", "widget-2"}, // Sorted by creation time (newest first)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.PaginationOptions{
				Page:    1,
				PerPage: 10,
				Filters: &models.FilterOptions{
					IsVisible: tt.isVisible,
				},
			}

			result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
			if err != nil {
				t.Fatalf("GetByUserIDWithFilters failed: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(result))
			}

			if total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, total)
			}

			// Verify correct widgets are returned
			for i, expectedID := range tt.expectedWidgets {
				if i >= len(result) {
					t.Errorf("Missing expected widget %s at index %d", expectedID, i)
					continue
				}
				if result[i].ID != expectedID {
					t.Errorf("Expected widget %s at index %d, got %s", expectedID, i, result[i].ID)
				}
			}
		})
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_SearchFilter(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create test widgets with different names
	widgets := []*models.Widget{
		createTestWidget("widget-1", userID, "Контактная форма", "lead-form", true, now.Add(-4*time.Hour)),
		createTestWidget("widget-2", userID, "Баннер рекламы", "banner", true, now.Add(-3*time.Hour)),
		createTestWidget("widget-3", userID, "Форма обратной связи", "lead-form", true, now.Add(-2*time.Hour)),
		createTestWidget("widget-4", userID, "Тест знаний", "quiz", true, now.Add(-1*time.Hour)),
		createTestWidget("widget-5", userID, "Contact Form", "lead-form", true, now),
	}

	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	tests := []struct {
		name            string
		searchTerm      string
		expectedCount   int
		expectedWidgets []string
	}{
		{
			name:            "search for 'форма' (case insensitive)",
			searchTerm:      "форма",
			expectedCount:   2,
			expectedWidgets: []string{"widget-3", "widget-1"}, // Sorted by creation time
		},
		{
			name:            "search for 'ФОРМА' (case insensitive)",
			searchTerm:      "ФОРМА",
			expectedCount:   2,
			expectedWidgets: []string{"widget-3", "widget-1"},
		},
		{
			name:            "search for 'баннер'",
			searchTerm:      "баннер",
			expectedCount:   1,
			expectedWidgets: []string{"widget-2"},
		},
		{
			name:            "search for 'contact' (English)",
			searchTerm:      "contact",
			expectedCount:   1,
			expectedWidgets: []string{"widget-5"},
		},
		{
			name:            "search for 'тест'",
			searchTerm:      "тест",
			expectedCount:   1,
			expectedWidgets: []string{"widget-4"},
		},
		{
			name:            "search for non-existent term",
			searchTerm:      "несуществующий",
			expectedCount:   0,
			expectedWidgets: []string{},
		},
		{
			name:            "empty search term",
			searchTerm:      "",
			expectedCount:   5,
			expectedWidgets: []string{"widget-5", "widget-4", "widget-3", "widget-2", "widget-1"},
		},
		{
			name:            "search with spaces",
			searchTerm:      "  форма  ",
			expectedCount:   2,
			expectedWidgets: []string{"widget-3", "widget-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.PaginationOptions{
				Page:    1,
				PerPage: 10,
				Filters: &models.FilterOptions{
					Search: tt.searchTerm,
				},
			}

			result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
			if err != nil {
				t.Fatalf("GetByUserIDWithFilters failed: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(result))
			}

			if total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, total)
			}

			// Verify correct widgets are returned
			for i, expectedID := range tt.expectedWidgets {
				if i >= len(result) {
					t.Errorf("Missing expected widget %s at index %d", expectedID, i)
					continue
				}
				if result[i].ID != expectedID {
					t.Errorf("Expected widget %s at index %d, got %s", expectedID, i, result[i].ID)
				}
			}
		})
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_CombinedFilters(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create test widgets with various combinations
	widgets := []*models.Widget{
		createTestWidget("widget-1", userID, "Контактная форма", "lead-form", true, now.Add(-5*time.Hour)),
		createTestWidget("widget-2", userID, "Скрытая форма", "lead-form", false, now.Add(-4*time.Hour)),
		createTestWidget("widget-3", userID, "Видимый баннер", "banner", true, now.Add(-3*time.Hour)),
		createTestWidget("widget-4", userID, "Скрытый баннер", "banner", false, now.Add(-2*time.Hour)),
		createTestWidget("widget-5", userID, "Форма опроса", "survey", true, now.Add(-1*time.Hour)),
		createTestWidget("widget-6", userID, "Тест форма", "quiz", true, now),
	}

	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	tests := []struct {
		name            string
		filters         *models.FilterOptions
		expectedCount   int
		expectedWidgets []string
	}{
		{
			name: "type + visibility filter",
			filters: &models.FilterOptions{
				Types:     []string{"lead-form"},
				IsVisible: boolPtr(true),
			},
			expectedCount:   1,
			expectedWidgets: []string{"widget-1"},
		},
		{
			name: "type + search filter",
			filters: &models.FilterOptions{
				Types:  []string{"lead-form", "survey"},
				Search: "форма",
			},
			expectedCount:   3,
			expectedWidgets: []string{"widget-5", "widget-2", "widget-1"}, // All contain "форма"
		},
		{
			name: "visibility + search filter",
			filters: &models.FilterOptions{
				IsVisible: boolPtr(true),
				Search:    "баннер",
			},
			expectedCount:   1,
			expectedWidgets: []string{"widget-3"},
		},
		{
			name: "all filters combined",
			filters: &models.FilterOptions{
				Types:     []string{"lead-form", "banner"},
				IsVisible: boolPtr(true),
				Search:    "форма",
			},
			expectedCount:   1,
			expectedWidgets: []string{"widget-1"}, // Only visible lead-form with "форма" in name
		},
		{
			name: "filters with no results",
			filters: &models.FilterOptions{
				Types:     []string{"quiz"},
				IsVisible: boolPtr(false),
				Search:    "форма",
			},
			expectedCount:   0,
			expectedWidgets: []string{},
		},
		{
			name: "multiple types with visibility",
			filters: &models.FilterOptions{
				Types:     []string{"banner", "survey"},
				IsVisible: boolPtr(true),
			},
			expectedCount:   2,
			expectedWidgets: []string{"widget-5", "widget-3"}, // Sorted by creation time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.PaginationOptions{
				Page:    1,
				PerPage: 10,
				Filters: tt.filters,
			}

			result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
			if err != nil {
				t.Fatalf("GetByUserIDWithFilters failed: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(result))
			}

			if total != tt.expectedCount {
				t.Errorf("Expected total count of %d, got %d", tt.expectedCount, total)
			}

			// Verify correct widgets are returned
			for i, expectedID := range tt.expectedWidgets {
				if i >= len(result) {
					t.Errorf("Missing expected widget %s at index %d", expectedID, i)
					continue
				}
				if result[i].ID != expectedID {
					t.Errorf("Expected widget %s at index %d, got %s", expectedID, i, result[i].ID)
				}
			}
		})
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_Pagination(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create 10 test widgets
	widgets := make([]*models.Widget, 10)
	for i := 0; i < 10; i++ {
		widgets[i] = createTestWidget(
			fmt.Sprintf("widget-%d", i+1),
			userID,
			fmt.Sprintf("Test Widget %d", i+1),
			"lead-form",
			true,
			now.Add(-time.Duration(10-i)*time.Hour), // Newer widgets have higher numbers
		)
	}

	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}

	tests := []struct {
		name          string
		page          int
		perPage       int
		expectedCount int
		expectedTotal int
		expectedFirst string
		expectedLast  string
	}{
		{
			name:          "first page",
			page:          1,
			perPage:       3,
			expectedCount: 3,
			expectedTotal: 10,
			expectedFirst: "widget-10", // Newest first
			expectedLast:  "widget-8",
		},
		{
			name:          "second page",
			page:          2,
			perPage:       3,
			expectedCount: 3,
			expectedTotal: 10,
			expectedFirst: "widget-7",
			expectedLast:  "widget-5",
		},
		{
			name:          "last page",
			page:          4,
			perPage:       3,
			expectedCount: 1,
			expectedTotal: 10,
			expectedFirst: "widget-1", // Oldest
			expectedLast:  "widget-1",
		},
		{
			name:          "page beyond results",
			page:          5,
			perPage:       3,
			expectedCount: 0,
			expectedTotal: 10,
			expectedFirst: "",
			expectedLast:  "",
		},
		{
			name:          "large page size",
			page:          1,
			perPage:       20,
			expectedCount: 10,
			expectedTotal: 10,
			expectedFirst: "widget-10",
			expectedLast:  "widget-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.PaginationOptions{
				Page:    tt.page,
				PerPage: tt.perPage,
				Filters: &models.FilterOptions{
					Types: []string{"lead-form"}, // Filter to ensure we get all widgets
				},
			}

			result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
			if err != nil {
				t.Fatalf("GetByUserIDWithFilters failed: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d widgets, got %d", tt.expectedCount, len(result))
			}

			if total != tt.expectedTotal {
				t.Errorf("Expected total count of %d, got %d", tt.expectedTotal, total)
			}

			if tt.expectedCount > 0 {
				if result[0].ID != tt.expectedFirst {
					t.Errorf("Expected first widget %s, got %s", tt.expectedFirst, result[0].ID)
				}
				if result[len(result)-1].ID != tt.expectedLast {
					t.Errorf("Expected last widget %s, got %s", tt.expectedLast, result[len(result)-1].ID)
				}
			}
		})
	}
}

func TestRedisWidgetRepository_GetByUserIDWithFilters_Performance(t *testing.T) {
	redisClient, cleanup := setupTestRedisForFiltering(t)
	defer cleanup()

	statsRepo := NewRedisStatsRepository(redisClient)
	repo := NewRedisWidgetRepository(redisClient, statsRepo)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create a larger dataset for performance testing
	numWidgets := 100
	widgets := make([]*models.Widget, numWidgets)
	types := []string{"lead-form", "banner", "quiz", "survey", "popup"}

	for i := 0; i < numWidgets; i++ {
		widgets[i] = createTestWidget(
			fmt.Sprintf("widget-%d", i+1),
			userID,
			fmt.Sprintf("Test Widget %d", i+1),
			types[i%len(types)], // Distribute across types
			i%2 == 0,            // Alternate visibility
			now.Add(-time.Duration(numWidgets-i)*time.Minute),
		)
	}

	// Create widgets
	start := time.Now()
	for _, widget := range widgets {
		err := repo.Create(ctx, widget)
		if err != nil {
			t.Fatalf("Failed to create widget %s: %v", widget.ID, err)
		}
	}
	createDuration := time.Since(start)
	t.Logf("Created %d widgets in %v", numWidgets, createDuration)

	// Test different filter scenarios for performance
	tests := []struct {
		name    string
		filters *models.FilterOptions
	}{
		{
			name:    "no filters",
			filters: nil,
		},
		{
			name: "type filter only",
			filters: &models.FilterOptions{
				Types: []string{"lead-form"},
			},
		},
		{
			name: "visibility filter only",
			filters: &models.FilterOptions{
				IsVisible: boolPtr(true),
			},
		},
		{
			name: "search filter only",
			filters: &models.FilterOptions{
				Search: "Widget",
			},
		},
		{
			name: "combined filters",
			filters: &models.FilterOptions{
				Types:     []string{"lead-form", "banner"},
				IsVisible: boolPtr(true),
				Search:    "Widget",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := models.PaginationOptions{
				Page:    1,
				PerPage: 20,
				Filters: tt.filters,
			}

			start := time.Now()
			result, total, err := repo.GetByUserIDWithFilters(ctx, userID, opts)
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("GetByUserIDWithFilters failed: %v", err)
			}

			t.Logf("Filter scenario '%s': returned %d/%d widgets in %v", tt.name, len(result), total, duration)

			// Performance assertion - should complete within reasonable time
			if duration > 1*time.Second {
				t.Errorf("Filter operation took too long: %v", duration)
			}

			// Verify results are reasonable
			if len(result) > opts.PerPage {
				t.Errorf("Returned more results than requested: %d > %d", len(result), opts.PerPage)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
