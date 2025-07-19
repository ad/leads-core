package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	exportService *services.ExportService
	validator     *validation.SchemaValidator
}

// NewWidgetHandler creates a new widget handler
func NewWidgetHandler(widgetService *services.WidgetService, exportService *services.ExportService, validator *validation.SchemaValidator) *WidgetHandler {
	return &WidgetHandler{
		widgetService: widgetService,
		exportService: exportService,
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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
	writeJSONResponse(w, http.StatusCreated, widget)
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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
	writeJSONResponse(w, http.StatusOK, models.WidgetsResponse{Widgets: widgets, Meta: meta})
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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
	writeJSONResponse(w, http.StatusOK, widget)
}

// UpdateWidget handles POST /widgets/{id}
func (h *WidgetHandler) UpdateWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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
	writeJSONResponse(w, http.StatusOK, widget)
}

// UpdateWidgetConfig handles PUT /widgets/{id}/config
func (h *WidgetHandler) UpdateWidgetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Extract widget ID from URL path
	widgetID := extractWidgetConfigID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Parse and validate request
	var req models.UpdateWidgetConfigRequest
	if err := h.validator.ValidateAndDecode(r, "widget-config-update", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeValidationErrors(w, valErr.Errors)
			return
		}
		logger.Error("Failed to decode widget config update request", map[string]interface{}{
			"action":    "update_widget_config",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})

		// Return 400 Bad Request for invalid JSON
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Update widget config
	widget, err := h.widgetService.UpdateWidgetConfig(r.Context(), widgetID, user.ID, &req)
	if err != nil {
		logger.Error("Failed to update widget config", map[string]interface{}{
			"action":    "update_widget_config",
			"user_id":   user.ID,
			"widget_id": widgetID,
			"error":     err.Error(),
		})
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to update widget config")
		}
		return
	}

	logger.Debug("Updated widget config successfully", map[string]interface{}{
		"action":    "update_widget_config",
		"user_id":   user.ID,
		"widget_id": widgetID,
	})
	writeJSONResponse(w, http.StatusOK, widget)
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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
	writeJSONResponse(w, http.StatusOK, stats)
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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

// ExportWidgetSubmissions handles GET /widgets/{id}/export
func (h *WidgetHandler) ExportWidgetSubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user from context
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetID(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Parse query parameters
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json" // Default format
	}

	// Validate format
	if format != "csv" && format != "json" && format != "xlsx" {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid format. Supported formats: csv, json, xlsx")
		return
	}

	// Parse time range parameters
	var from, to *time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if parsedFrom, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &parsedFrom
		} else {
			writeErrorResponse(w, http.StatusBadRequest, "Invalid 'from' date format. Use RFC3339 format (e.g., 2023-01-01T00:00:00Z)")
			return
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if parsedTo, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &parsedTo
		} else {
			writeErrorResponse(w, http.StatusBadRequest, "Invalid 'to' date format. Use RFC3339 format (e.g., 2023-12-31T23:59:59Z)")
			return
		}
	}

	// Create export options
	options := models.ExportOptions{
		Format: format,
		From:   from,
		To:     to,
	}

	// Export submissions using export service
	data, filename, err := h.exportService.ExportSubmissions(r.Context(), widgetID, user.ID, options)
	if err != nil {
		logger.Error("Failed to export widget submissions", map[string]interface{}{
			"action":    "export_widget_submissions",
			"widget_id": widgetID,
			"user_id":   user.ID,
			"format":    format,
			"error":     err.Error(),
		})

		if err.Error() == "unauthorized" {
			writeErrorResponse(w, http.StatusForbidden, "Access denied")
			return
		}

		if err.Error() == "widget not found" {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, "Failed to export submissions")
		return
	}

	// Set appropriate headers based on format
	var contentType string
	switch format {
	case "csv":
		contentType = "text/csv"
	case "json":
		contentType = "application/json"
	case "xlsx":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	logger.Info("Widget submissions exported successfully", map[string]interface{}{
		"action":    "export_widget_submissions",
		"widget_id": widgetID,
		"user_id":   user.ID,
		"format":    format,
		"filename":  filename,
		"size":      len(data),
	})

	w.Write(data)
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
		writeErrorResponse(w, http.StatusUnauthorized, "User not found")
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

// extractWidgetConfigID extracts widget ID from config URL path
func extractWidgetConfigID(path string) string {
	// Extract from /api/v1/widgets/{id}/config
	trimmedPath := strings.TrimPrefix(path, "/api/v1/widgets/")
	parts := strings.SplitN(trimmedPath, "/", 2)
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
