package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ad/leads-core/internal/auth"
	customErrors "github.com/ad/leads-core/internal/errors"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/validation"
	"github.com/ad/leads-core/pkg/logger"
)

// WidgetHandler handles widget-related HTTP requests
type WidgetHandler struct {
	widgetService *services.WidgetService
	validator     *validation.SchemaValidator
}

// NewWidgetHandler creates a new widget handler
func NewWidgetHandler(widgetService *services.WidgetService, validator *validation.SchemaValidator) *WidgetHandler {
	return &WidgetHandler{
		widgetService: widgetService,
		validator:     validator,
	}
}

// CreateWidget handles POST /widgets
func (h *WidgetHandler) CreateWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Parse and validate request
	var req models.CreateWidgetRequest
	if err := h.validator.ValidateAndDecode(r, "widget-create", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeValidationErrors(w, valErr.Errors)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Create widget
	widget, err := h.widgetService.CreateWidget(r.Context(), user.ID, req)
	if err != nil {
		logger.Error("Failed to create widget", map[string]interface{}{
			"action":  "create_widget",
			"user_id": user.ID,
			"error":   err.Error(),
		})
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Debug("Widget created successfully", map[string]interface{}{
		"action":    "create_widget",
		"user_id":   user.ID,
		"widget_id": widget.ID,
		"type":      widget.Type,
	})
	writeJSONResponse(w, http.StatusCreated, models.Response{Data: widget})
}

// GetWidgets handles GET /widgets
func (h *WidgetHandler) GetWidgets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Parse pagination parameters
	opts := parsePaginationOptions(r)

	// Get widgets
	widgets, total, err := h.widgetService.GetUserWidgets(r.Context(), user.ID, opts)
	if err != nil {
		logger.Error("Failed to get widgets", map[string]interface{}{
			"action":  "get_widgets",
			"user_id": user.ID,
			"error":   err.Error(),
		})
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get widgets")
		return
	}

	// Calculate pagination metadata
	meta := &models.Meta{
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Total:   total,
		HasMore: len(widgets) == opts.PerPage,
	}

	logger.Debug("Retrieved widgets successfully", map[string]interface{}{
		"action":  "get_widgets",
		"user_id": user.ID,
		"count":   len(widgets),
	})
	writeJSONResponse(w, http.StatusOK, models.Response{Data: widgets, Meta: meta})
}

// GetWidget handles GET /widgets/{id}
func (h *WidgetHandler) GetWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Get widget
	widget, err := h.widgetService.GetWidget(r.Context(), widgetID, user.ID)
	if err != nil {
		logger.Error("Failed to get widget", map[string]interface{}{
			"action":    "get_widget",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get widget")
		}
		return
	}

	logger.Debug("Retrieved widget successfully", map[string]interface{}{
		"action":    "get_widget",
		"user_id":   user.ID,
		"widget_id": widgetID,
	})
	writeJSONResponse(w, http.StatusOK, models.Response{Data: widget})
}

// UpdateWidget handles PUT /widgets/{id}
func (h *WidgetHandler) UpdateWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Parse and validate request
	var req models.UpdateWidgetRequest
	if err := h.validator.ValidateAndDecode(r, "widget-update", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeValidationErrors(w, valErr.Errors)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Update widget
	widget, err := h.widgetService.UpdateWidget(r.Context(), widgetID, user.ID, req)
	if err != nil {
		logger.Error("Failed to update widget", map[string]interface{}{
			"action":    "update_widget",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	logger.Debug("Updated widget successfully", map[string]interface{}{
		"action":    "update_widget",
		"user_id":   user.ID,
		"widget_id": widgetID,
	})
	writeJSONResponse(w, http.StatusOK, models.Response{Data: widget})
}

// DeleteWidget handles DELETE /widgets/{id}
func (h *WidgetHandler) DeleteWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Delete widget
	if err := h.widgetService.DeleteWidget(r.Context(), widgetID, user.ID); err != nil {
		logger.Error("Failed to delete widget", map[string]interface{}{
			"action":    "delete_widget",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete widget")
		}
		return
	}

	logger.Debug("Deleted widget successfully", map[string]interface{}{
		"action":    "delete_widget",
		"user_id":   user.ID,
		"widget_id": widgetID,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GetWidgetStats handles GET /widgets/{id}/stats
func (h *WidgetHandler) GetWidgetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Get stats
	stats, err := h.widgetService.GetWidgetStats(r.Context(), widgetID, user.ID)
	if err != nil {
		logger.Error("Failed to get widget stats", map[string]interface{}{
			"action":    "get_widget_stats",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get widget stats")
		}
		return
	}

	logger.Debug("Retrieved widget stats successfully", map[string]interface{}{
		"action":    "get_widget_stats",
		"user_id":   user.ID,
		"widget_id": widgetID,
	})
	writeJSONResponse(w, http.StatusOK, models.Response{Data: stats})
}

// GetWidgetSubmissions handles GET /widgets/{id}/submissions
func (h *WidgetHandler) GetWidgetSubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Parse pagination parameters
	opts := parsePaginationOptions(r)

	// Get submissions
	submissions, total, err := h.widgetService.GetWidgetSubmissions(r.Context(), widgetID, user.ID, opts)
	if err != nil {
		logger.Error("Failed to get widget submissions", map[string]interface{}{
			"action":    "get_widget_submissions",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get widget submissions")
		}
		return
	}

	// Calculate pagination metadata
	meta := &models.Meta{
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Total:   total,
		HasMore: len(submissions) == opts.PerPage,
	}

	logger.Debug("Retrieved widget submissions successfully", map[string]interface{}{
		"action":    "get_widget_submissions",
		"user_id":   user.ID,
		"widget_id": widgetID,
		"count":     len(submissions),
	})
	writeJSONResponse(w, http.StatusOK, models.Response{Data: submissions, Meta: meta})
}

// GetWidgetsSummary handles GET /widgets/summary
func (h *WidgetHandler) GetWidgetsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Get widgets summary
	summary, err := h.widgetService.GetWidgetsSummary(r.Context(), user.ID)
	if err != nil {
		logger.Error("Failed to get widgets summary", map[string]interface{}{
			"action":  "get_widgets_summary",
			"user_id": user.ID,
			"error":   err.Error(),
		})
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get widgets summary")
		return
	}

	logger.Debug("Retrieved widgets summary successfully", map[string]interface{}{
		"action":  "get_widgets_summary",
		"user_id": user.ID,
	})
	writeJSONResponse(w, http.StatusOK, models.Response{Data: summary})
}

// parsePaginationOptions parses pagination parameters from request
func parsePaginationOptions(r *http.Request) models.PaginationOptions {
	page := 1
	perPage := 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	// Also support 'limit' parameter as alias for per_page (for submissions API)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			perPage = l
		}
	}

	return models.PaginationOptions{
		Page:    page,
		PerPage: perPage,
	}
}

// extractWidgetID extracts widget ID from URL path
func extractWidgetID(path string) string {
	// Trim the prefix and then split to get the ID
	// e.g., /widgets/{id} or /widgets/{id}/stats
	trimmedPath := strings.TrimPrefix(path, "/widgets/")
	parts := strings.SplitN(trimmedPath, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
