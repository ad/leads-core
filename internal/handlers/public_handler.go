package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/validation"
)

// PublicHandler handles public (non-authenticated) endpoints
type PublicHandler struct {
	widgetService *services.WidgetService
	validator     *validation.SchemaValidator
}

// NewPublicHandler creates a new public handler
func NewPublicHandler(widgetService *services.WidgetService, validator *validation.SchemaValidator) *PublicHandler {
	return &PublicHandler{
		widgetService: widgetService,
		validator:     validator,
	}
}

// SubmitWidget handles POST /widgets/{id}/submit
func (h *PublicHandler) SubmitWidget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetIDFromSubmitPath(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Parse and validate request
	var req models.SubmissionRequest
	if err := h.validator.ValidateAndDecode(r, "submission", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeErrorResponse(w, http.StatusBadRequest, "Validation error", valErr.Errors)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Submit widget
	submission, err := h.widgetService.SubmitWidget(r.Context(), widgetID, req)
	if err != nil {
		log.Printf("action=submit_widget widget_id=%s error=%q", widgetID, err.Error())
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else if strings.Contains(err.Error(), "disabled") {
			writeErrorResponse(w, http.StatusForbidden, "Widget is disabled")
		} else {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	log.Printf("action=submit_widget widget_id=%s submission_id=%s", widgetID, submission.ID)
	writeJSONResponse(w, http.StatusCreated, models.Response{Data: submission})
}

// RegisterEvent handles POST /widgets/{id}/events
func (h *PublicHandler) RegisterEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract widget ID from URL
	widgetID := extractWidgetIDFromEventPath(r.URL.Path)
	if widgetID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Widget ID is required")
		return
	}

	// Parse and validate request
	var req models.EventRequest
	if err := h.validator.ValidateAndDecode(r, "event", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeErrorResponse(w, http.StatusBadRequest, "Validation error", valErr.Errors)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate event type
	if req.Type != "view" && req.Type != "close" {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid event type. Must be 'view' or 'close'")
		return
	}

	// Register event
	if err := h.widgetService.RegisterWidgetEvent(r.Context(), widgetID, req.Type); err != nil {
		log.Printf("action=register_event widget_id=%s type=%s error=%q", widgetID, req.Type, err.Error())
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Widget not found")
		} else if strings.Contains(err.Error(), "disabled") {
			writeErrorResponse(w, http.StatusForbidden, "Widget is disabled")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to register event")
		}
		return
	}

	log.Printf("action=register_event widget_id=%s type=%s", widgetID, req.Type)
	w.WriteHeader(http.StatusNoContent)
}

// extractWidgetIDFromSubmitPath extracts widget ID from paths like /widgets/{id}/submit
func extractWidgetIDFromSubmitPath(path string) string {
	// Remove leading/trailing slashes and split
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expected format: ["widgets", "{id}", "submit"]
	if len(parts) == 3 && parts[0] == "widgets" && parts[2] == "submit" {
		return parts[1]
	}
	return ""
}

// extractWidgetIDFromEventPath extracts widget ID from paths like /widgets/{id}/events
func extractWidgetIDFromEventPath(path string) string {
	// Remove leading/trailing slashes and split
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expected format: ["widgets", "{id}", "events"]
	if len(parts) == 3 && parts[0] == "widgets" && parts[2] == "events" {
		return parts[1]
	}
	return ""
}
