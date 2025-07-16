package models

import (
	"encoding/json"
	"strconv"
	"time"
)

// User represents user data extracted from JWT token
type User struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Plan     string `json:"plan,omitempty"` // "free", "pro", etc.
}

// Form represents a form created by a user
type Form struct {
	ID        string                 `json:"id"`
	OwnerID   string                 `json:"owner_id"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	Enabled   bool                   `json:"enabled"`
	Fields    map[string]interface{} `json:"fields"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Stats     *FormStats             `json:"stats,omitempty"`
}

// Submission represents a submission to a form
type Submission struct {
	ID        string                 `json:"id"`
	FormID    string                 `json:"form_id"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	TTL       time.Duration          `json:"ttl,omitempty"`
}

// FormStats represents statistics for a form
type FormStats struct {
	FormID   string    `json:"form_id"`
	Views    int64     `json:"views"`
	Submits  int64     `json:"submits"`
	Closes   int64     `json:"closes"`
	LastView time.Time `json:"last_view,omitempty"`
}

// CreateFormRequest represents request data for creating a form
type CreateFormRequest struct {
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Fields  map[string]interface{} `json:"fields"`
}

// UpdateFormRequest represents request data for updating a form
type UpdateFormRequest struct {
	Type    *string                `json:"type,omitempty"`
	Name    *string                `json:"name,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// SubmissionRequest represents request data for creating a submission
type SubmissionRequest struct {
	Data map[string]interface{} `json:"data"`
}

// EventRequest represents request data for form events
type EventRequest struct {
	Type string `json:"type"` // "view", "close"
}

// PaginationOptions represents pagination parameters
type PaginationOptions struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
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

// FormsSummary represents a summary of user's forms
type FormsSummary struct {
	TotalForms       int `json:"total_forms"`
	ActiveForms      int `json:"active_forms"`
	DisabledForms    int `json:"disabled_forms"`
	TotalViews       int `json:"total_views"`
	TotalSubmissions int `json:"total_submissions"`
}

// ToRedisHash converts Form to map for Redis HSET
func (f *Form) ToRedisHash() map[string]interface{} {
	fieldsJSON, _ := json.Marshal(f.Fields)
	return map[string]interface{}{
		"id":         f.ID,
		"owner_id":   f.OwnerID,
		"type":       f.Type,
		"name":       f.Name,
		"enabled":    strconv.FormatBool(f.Enabled),
		"fields":     string(fieldsJSON),
		"created_at": f.CreatedAt.Unix(),
		"updated_at": f.UpdatedAt.Unix(),
	}
}

// FromRedisHash converts Redis hash to Form
func (f *Form) FromRedisHash(hash map[string]string) error {
	f.ID = hash["id"]
	f.OwnerID = hash["owner_id"]
	f.Type = hash["type"]
	f.Name = hash["name"]
	f.Enabled = hash["enabled"] == "true"

	if fieldsStr, ok := hash["fields"]; ok && fieldsStr != "" {
		if err := json.Unmarshal([]byte(fieldsStr), &f.Fields); err != nil {
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
		"form_id":    s.FormID,
		"data":       string(dataJSON),
		"created_at": s.CreatedAt.Unix(),
	}
}

// FromRedisHash converts Redis hash to Submission
func (s *Submission) FromRedisHash(hash map[string]string) error {
	s.ID = hash["id"]
	s.FormID = hash["form_id"]

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
