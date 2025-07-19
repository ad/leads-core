package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// User represents user data extracted from JWT token
type User struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Plan     string `json:"plan,omitempty"` // "free", "pro", etc.
}

// Widget represents a widget created by a user
type Widget struct {
	ID        string                 `json:"id"`
	OwnerID   string                 `json:"owner_id"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	IsVisible bool                   `json:"isVisible"`
	Config    map[string]interface{} `json:"config"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Stats     *WidgetStats           `json:"stats,omitempty"`
}

// Submission represents a submission to a widget
type Submission struct {
	ID        string                 `json:"id"`
	WidgetID  string                 `json:"widget_id"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	TTL       time.Duration          `json:"ttl,omitempty"`
}

// WidgetStats represents statistics for a widget
type WidgetStats struct {
	WidgetID string    `json:"widget_id"`
	Views    int64     `json:"views"`
	Submits  int64     `json:"submits"`
	Closes   int64     `json:"closes"`
	LastView time.Time `json:"last_view,omitempty"`
}

// CreateWidgetRequest represents request data for creating a widget
type CreateWidgetRequest struct {
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	IsVisible bool                   `json:"isVisible"`
	Config    map[string]interface{} `json:"config"`
}

// UpdateWidgetRequest represents request data for updating a widget
type UpdateWidgetRequest struct {
	Type      *string `json:"type,omitempty"`
	Name      *string `json:"name,omitempty"`
	IsVisible *bool   `json:"isVisible,omitempty"`
}

// UpdateWidgetConfigRequest represents request data for updating widget config
type UpdateWidgetConfigRequest struct {
	Config map[string]interface{} `json:"config"`
}

// SubmissionRequest represents request data for creating a submission
type SubmissionRequest struct {
	Data map[string]interface{} `json:"data"`
}

// EventRequest represents request data for widget events
type EventRequest struct {
	Type string `json:"type"` // "view", "close"
}

// FilterOptions represents filtering parameters for widgets
type FilterOptions struct {
	Types     []string `json:"types,omitempty"`     // Filter by widget types
	IsVisible *bool    `json:"isVisible,omitempty"` // Filter by visibility status (nil = all)
	Search    string   `json:"search,omitempty"`    // Search by widget name
}

// PaginationOptions represents pagination parameters
type PaginationOptions struct {
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
	Filters *FilterOptions `json:"filters,omitempty"` // Optional filtering parameters
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data interface{} `json:"data"`
	Meta Meta        `json:"meta"`
}

// Meta represents pagination metadata
type Meta struct {
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// WidgetsResponse represents a response containing multiple widgets
type WidgetsResponse struct {
	Widgets interface{} `json:"widgets,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Response represents a standard API response
type Response struct {
	Data interface{} `json:"data,omitempty"`
	Meta *Meta       `json:"meta,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string      `json:"error"`
	Details interface{} `json:"details,omitempty"`
}

// WidgetsSummary represents a summary of user's widgets
type WidgetsSummary struct {
	TotalWidgets     int `json:"total_widgets"`
	ActiveWidgets    int `json:"active_widgets"`
	DisabledWidgets  int `json:"disabled_widgets"`
	TotalViews       int `json:"total_views"`
	TotalSubmissions int `json:"total_submissions"`
}

// ToRedisHash converts Widget to map for Redis HSET
func (f *Widget) ToRedisHash() map[string]interface{} {
	configJSON, _ := json.Marshal(f.Config)
	return map[string]interface{}{
		"id":         f.ID,
		"owner_id":   f.OwnerID,
		"type":       f.Type,
		"name":       f.Name,
		"isVisible":  strconv.FormatBool(f.IsVisible),
		"config":     string(configJSON),
		"created_at": f.CreatedAt.Unix(),
		"updated_at": f.UpdatedAt.Unix(),
	}
}

// FromRedisHash converts Redis hash to Widget
func (f *Widget) FromRedisHash(hash map[string]string) error {
	f.ID = hash["id"]
	f.OwnerID = hash["owner_id"]
	f.Type = hash["type"]
	f.Name = hash["name"]
	f.IsVisible = hash["isVisible"] == "true"

	if configStr, ok := hash["config"]; ok && configStr != "" {
		if err := json.Unmarshal([]byte(configStr), &f.Config); err != nil {
			return err
		}
	}

	if createdAtStr, ok := hash["created_at"]; ok && createdAtStr != "" {
		if timestamp, err := strconv.ParseInt(createdAtStr, 10, 64); err == nil {
			f.CreatedAt = time.Unix(timestamp, 0)
		}
	}

	if updatedAtStr, ok := hash["updated_at"]; ok && updatedAtStr != "" {
		if timestamp, err := strconv.ParseInt(updatedAtStr, 10, 64); err == nil {
			f.UpdatedAt = time.Unix(timestamp, 0)
		}
	}

	return nil
}

// ToRedisHash converts Submission to map for Redis HSET
func (s *Submission) ToRedisHash() map[string]interface{} {
	dataJSON, _ := json.Marshal(s.Data)
	return map[string]interface{}{
		"id":         s.ID,
		"widget_id":  s.WidgetID,
		"data":       string(dataJSON),
		"created_at": s.CreatedAt.Unix(),
	}
}

// FromRedisHash converts Redis hash to Submission
func (s *Submission) FromRedisHash(hash map[string]string) error {
	s.ID = hash["id"]
	s.WidgetID = hash["widget_id"]

	if dataStr, ok := hash["data"]; ok && dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), &s.Data); err != nil {
			return err
		}
	}

	if createdAtStr, ok := hash["created_at"]; ok && createdAtStr != "" {
		if timestamp, err := strconv.ParseInt(createdAtStr, 10, 64); err == nil {
			s.CreatedAt = time.Unix(timestamp, 0)
		}
	}

	return nil
}

// UpdateTTLRequest represents request data for updating TTL
type UpdateTTLRequest struct {
	TTLDays int `json:"ttl_days"`
}

// MetricsResponse represents metrics data
type MetricsResponse struct {
	Timestamp time.Time      `json:"timestamp"`
	Uptime    time.Duration  `json:"uptime"`
	Memory    MemoryMetrics  `json:"memory"`
	Runtime   RuntimeMetrics `json:"runtime"`
}

// MemoryMetrics represents memory usage metrics
type MemoryMetrics struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"total_alloc"`
	Sys        uint64 `json:"sys"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapSys    uint64 `json:"heap_sys"`
}

// RuntimeMetrics represents runtime metrics
type RuntimeMetrics struct {
	Goroutines int       `json:"goroutines"`
	GCRuns     uint32    `json:"gc_runs"`
	NextGC     uint64    `json:"next_gc"`
	LastGC     time.Time `json:"last_gc"`
}

// ExportRequest represents request data for exporting submissions
type ExportRequest struct {
	Format string     `json:"format"` // "csv", "json", "xlsx"
	From   *time.Time `json:"from,omitempty"`
	To     *time.Time `json:"to,omitempty"`
}

// ExportOptions represents options for exporting submissions
type ExportOptions struct {
	Format string
	From   *time.Time
	To     *time.Time
}

// ValidateFilterOptions validates filter options and returns cleaned version
func ValidateFilterOptions(filters *FilterOptions) *FilterOptions {
	if filters == nil {
		return nil
	}

	validated := &FilterOptions{
		Types:     make([]string, 0),
		IsVisible: filters.IsVisible,
		Search:    strings.TrimSpace(filters.Search),
	}

	// Validate and clean widget types
	validTypes := map[string]bool{
		"lead-form": true,
		"banner":    true,
		"quiz":      true,
		"survey":    true,
		"popup":     true,
	}

	for _, widgetType := range filters.Types {
		cleanType := strings.TrimSpace(strings.ToLower(widgetType))
		if cleanType != "" && validTypes[cleanType] {
			validated.Types = append(validated.Types, cleanType)
		}
	}

	return validated
}

// HasFilters returns true if any filters are applied
func (f *FilterOptions) HasFilters() bool {
	if f == nil {
		return false
	}
	return len(f.Types) > 0 || f.IsVisible != nil || f.Search != ""
}

// HasTypeFilter returns true if type filter is applied
func (f *FilterOptions) HasTypeFilter() bool {
	return f != nil && len(f.Types) > 0
}

// HasVisibilityFilter returns true if visibility filter is applied
func (f *FilterOptions) HasVisibilityFilter() bool {
	return f != nil && f.IsVisible != nil
}

// HasSearchFilter returns true if search filter is applied
func (f *FilterOptions) HasSearchFilter() bool {
	return f != nil && f.Search != ""
}
