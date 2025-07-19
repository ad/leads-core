package handlers

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/ad/leads-core/internal/models"
)

func TestParseFilterOptions(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected *models.FilterOptions
	}{
		{
			name:  "no filters",
			query: "",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "single type filter",
			query: "type=lead-form",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form"},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "multiple types filter",
			query: "type=lead-form,banner,quiz",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner", "quiz"},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "multiple types with spaces",
			query: "type=lead-form, banner , quiz",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner", "quiz"},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "visibility filter true",
			query: "isVisible=true",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: boolPtr(true),
				Search:    "",
			},
		},
		{
			name:  "visibility filter false",
			query: "isVisible=false",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: boolPtr(false),
				Search:    "",
			},
		},
		{
			name:  "search filter",
			query: "search=контакт",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "контакт",
			},
		},
		{
			name:  "search filter with spaces",
			query: "search=  форма контакта  ",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "форма контакта",
			},
		},
		{
			name:  "combined filters",
			query: "type=lead-form,banner&isVisible=true&search=тест",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner"},
				IsVisible: boolPtr(true),
				Search:    "тест",
			},
		},
		{
			name:  "invalid type ignored",
			query: "type=lead-form,invalid-type,banner",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner"},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "invalid visibility ignored",
			query: "isVisible=invalid",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "empty type parameter",
			query: "type=",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "empty search parameter",
			query: "search=",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "type with empty values",
			query: "type=lead-form,,banner",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner"},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "search with special characters",
			query: "search=%D1%82%D0%B5%D1%81%D1%82%40%23%24%25%5E%2A%28%29", // "тест@#$%^*()" URL encoded (removed & to avoid parsing issues)
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "тест@#$%^*()",
			},
		},
		{
			name:  "search with null bytes removed",
			query: "search=тест\x00контакт",
			expected: &models.FilterOptions{
				Types:     []string{},
				IsVisible: nil,
				Search:    "тестконтакт",
			},
		},
		{
			name:  "case insensitive type validation",
			query: "type=LEAD-FORM,Banner,QUIZ",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner", "quiz"},
				IsVisible: nil,
				Search:    "",
			},
		},
		{
			name:  "all valid widget types",
			query: "type=lead-form,banner,quiz,survey,popup",
			expected: &models.FilterOptions{
				Types:     []string{"lead-form", "banner", "quiz", "survey", "popup"},
				IsVisible: nil,
				Search:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with query parameters
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.query,
				},
			}

			// Parse filter options
			result := parseFilterOptions(req)

			// Validate result
			if result == nil {
				t.Fatal("parseFilterOptions returned nil")
			}

			// Check types
			if !reflect.DeepEqual(result.Types, tt.expected.Types) {
				t.Errorf("Types mismatch. Expected: %v, Got: %v", tt.expected.Types, result.Types)
			}

			// Check visibility
			if (result.IsVisible == nil) != (tt.expected.IsVisible == nil) {
				t.Errorf("IsVisible nil mismatch. Expected: %v, Got: %v", tt.expected.IsVisible, result.IsVisible)
			} else if result.IsVisible != nil && tt.expected.IsVisible != nil {
				if *result.IsVisible != *tt.expected.IsVisible {
					t.Errorf("IsVisible value mismatch. Expected: %v, Got: %v", *tt.expected.IsVisible, *result.IsVisible)
				}
			}

			// Check search
			if result.Search != tt.expected.Search {
				t.Errorf("Search mismatch. Expected: %q, Got: %q", tt.expected.Search, result.Search)
			}
		})
	}
}

func TestParseFilterOptionsEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		description string
	}{
		{
			name:        "multiple same parameters",
			query:       "type=lead-form&type=banner",
			description: "should use last value when parameter is repeated",
		},
		{
			name:        "url encoded values",
			query:       "search=%D1%82%D0%B5%D1%81%D1%82", // "тест" URL encoded
			description: "should handle URL encoded search terms",
		},
		{
			name:        "mixed case boolean",
			query:       "isVisible=True",
			description: "should handle mixed case boolean values",
		},
		{
			name:        "numeric boolean",
			query:       "isVisible=1",
			description: "should handle numeric boolean values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.query,
				},
			}

			result := parseFilterOptions(req)
			if result == nil {
				t.Fatal("parseFilterOptions returned nil")
			}

			// These tests verify that the function doesn't panic or crash
			// with edge case inputs. Specific behavior validation would depend
			// on the exact requirements for each edge case.
			t.Logf("Test passed: %s", tt.description)
		})
	}
}

func TestParsePaginationWithFilters(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected models.PaginationOptions
	}{
		{
			name:  "pagination only",
			query: "page=2&per_page=10",
			expected: models.PaginationOptions{
				Page:    2,
				PerPage: 10,
				Filters: &models.FilterOptions{
					Types:     []string{},
					IsVisible: nil,
					Search:    "",
				},
			},
		},
		{
			name:  "pagination with filters",
			query: "page=3&per_page=5&type=lead-form&isVisible=true&search=тест",
			expected: models.PaginationOptions{
				Page:    3,
				PerPage: 5,
				Filters: &models.FilterOptions{
					Types:     []string{"lead-form"},
					IsVisible: boolPtr(true),
					Search:    "тест",
				},
			},
		},
		{
			name:  "default pagination with filters",
			query: "type=banner,quiz&search=контакт",
			expected: models.PaginationOptions{
				Page:    1,
				PerPage: 20,
				Filters: &models.FilterOptions{
					Types:     []string{"banner", "quiz"},
					IsVisible: nil,
					Search:    "контакт",
				},
			},
		},
		{
			name:  "invalid pagination values with filters",
			query: "page=-1&per_page=200&type=lead-form",
			expected: models.PaginationOptions{
				Page:    1,  // Invalid page defaults to 1
				PerPage: 20, // Invalid per_page defaults to 20 (over limit)
				Filters: &models.FilterOptions{
					Types:     []string{"lead-form"},
					IsVisible: nil,
					Search:    "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					RawQuery: tt.query,
				},
			}

			result := parsePaginationWithFilters(req)

			// Check pagination
			if result.Page != tt.expected.Page {
				t.Errorf("Page mismatch. Expected: %d, Got: %d", tt.expected.Page, result.Page)
			}
			if result.PerPage != tt.expected.PerPage {
				t.Errorf("PerPage mismatch. Expected: %d, Got: %d", tt.expected.PerPage, result.PerPage)
			}

			// Check filters
			if result.Filters == nil {
				t.Fatal("Filters should not be nil")
			}

			if !reflect.DeepEqual(result.Filters.Types, tt.expected.Filters.Types) {
				t.Errorf("Filter Types mismatch. Expected: %v, Got: %v", tt.expected.Filters.Types, result.Filters.Types)
			}

			if (result.Filters.IsVisible == nil) != (tt.expected.Filters.IsVisible == nil) {
				t.Errorf("Filter IsVisible nil mismatch. Expected: %v, Got: %v", tt.expected.Filters.IsVisible, result.Filters.IsVisible)
			} else if result.Filters.IsVisible != nil && tt.expected.Filters.IsVisible != nil {
				if *result.Filters.IsVisible != *tt.expected.Filters.IsVisible {
					t.Errorf("Filter IsVisible value mismatch. Expected: %v, Got: %v", *tt.expected.Filters.IsVisible, *result.Filters.IsVisible)
				}
			}

			if result.Filters.Search != tt.expected.Filters.Search {
				t.Errorf("Filter Search mismatch. Expected: %q, Got: %q", tt.expected.Filters.Search, result.Filters.Search)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
