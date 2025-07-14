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
	formService *services.FormService
	validator   *validation.SchemaValidator
}

// NewPublicHandler creates a new public handler
func NewPublicHandler(formService *services.FormService, validator *validation.SchemaValidator) *PublicHandler {
	return &PublicHandler{
		formService: formService,
		validator:   validator,
	}
}

// SubmitForm handles POST /forms/{id}/submit
func (h *PublicHandler) SubmitForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract form ID from URL
	formID := extractFormIDFromSubmitPath(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
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

	// Submit form
	submission, err := h.formService.SubmitForm(r.Context(), formID, req)
	if err != nil {
		log.Printf("action=submit_form form_id=%s error=%q", formID, err.Error())
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else if strings.Contains(err.Error(), "disabled") {
			writeErrorResponse(w, http.StatusForbidden, "Form is disabled")
		} else {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	log.Printf("action=submit_form form_id=%s submission_id=%s", formID, submission.ID)
	writeJSONResponse(w, http.StatusCreated, models.Response{Data: submission})
}

// RegisterEvent handles POST /forms/{id}/events
func (h *PublicHandler) RegisterEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract form ID from URL
	formID := extractFormIDFromEventPath(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
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
	if err := h.formService.RegisterFormEvent(r.Context(), formID, req.Type); err != nil {
		log.Printf("action=register_event form_id=%s type=%s error=%q", formID, req.Type, err.Error())
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else if strings.Contains(err.Error(), "disabled") {
			writeErrorResponse(w, http.StatusForbidden, "Form is disabled")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to register event")
		}
		return
	}

	log.Printf("action=register_event form_id=%s type=%s", formID, req.Type)
	w.WriteHeader(http.StatusNoContent)
}

// extractFormIDFromSubmitPath extracts form ID from paths like /forms/{id}/submit
func extractFormIDFromSubmitPath(path string) string {
	// Remove leading/trailing slashes and split
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expected format: ["forms", "{id}", "submit"]
	if len(parts) == 3 && parts[0] == "forms" && parts[2] == "submit" {
		return parts[1]
	}
	return ""
}

// extractFormIDFromEventPath extracts form ID from paths like /forms/{id}/events
func extractFormIDFromEventPath(path string) string {
	// Remove leading/trailing slashes and split
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expected format: ["forms", "{id}", "events"]
	if len(parts) == 3 && parts[0] == "forms" && parts[2] == "events" {
		return parts[1]
	}
	return ""
}
