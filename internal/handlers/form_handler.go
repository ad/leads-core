package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ad/leads-core/internal/auth"
	customErrors "github.com/ad/leads-core/internal/errors"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/validation"
)

// FormHandler handles form-related HTTP requests
type FormHandler struct {
	formService *services.FormService
	validator   *validation.SchemaValidator
}

// NewFormHandler creates a new form handler
func NewFormHandler(formService *services.FormService, validator *validation.SchemaValidator) *FormHandler {
	return &FormHandler{
		formService: formService,
		validator:   validator,
	}
}

// CreateForm handles POST /forms
func (h *FormHandler) CreateForm(w http.ResponseWriter, r *http.Request) {
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
	var req models.CreateFormRequest
	if err := h.validator.ValidateAndDecode(r, "form-create", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeErrorResponse(w, http.StatusBadRequest, "Validation error", valErr.Errors)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Create form
	form, err := h.formService.CreateForm(r.Context(), user.ID, req)
	if err != nil {
		log.Printf("action=create_form user_id=%s error=%q", user.ID, err.Error())
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("action=create_form user_id=%s form_id=%s type=%s", user.ID, form.ID, form.Type)
	writeJSONResponse(w, http.StatusCreated, models.Response{Data: form})
}

// GetForms handles GET /forms
func (h *FormHandler) GetForms(w http.ResponseWriter, r *http.Request) {
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

	// Get forms
	forms, total, err := h.formService.GetUserForms(r.Context(), user.ID, opts)
	if err != nil {
		log.Printf("action=get_forms user_id=%s error=%q", user.ID, err.Error())
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get forms")
		return
	}

	// Calculate pagination metadata
	meta := &models.Meta{
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Total:   total,
		HasMore: len(forms) == opts.PerPage,
	}

	log.Printf("action=get_forms user_id=%s count=%d", user.ID, len(forms))
	writeJSONResponse(w, http.StatusOK, models.Response{Data: forms, Meta: meta})
}

// GetForm handles GET /forms/{id}
func (h *FormHandler) GetForm(w http.ResponseWriter, r *http.Request) {
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

	// Extract form ID from URL
	formID := extractFormID(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
		return
	}

	// Get form
	form, err := h.formService.GetForm(r.Context(), formID, user.ID)
	if err != nil {
		log.Printf("action=get_form user_id=%s form_id=%s error=%q", user.ID, formID, err.Error())
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get form")
		}
		return
	}

	log.Printf("action=get_form user_id=%s form_id=%s", user.ID, formID)
	writeJSONResponse(w, http.StatusOK, models.Response{Data: form})
}

// UpdateForm handles PUT /forms/{id}
func (h *FormHandler) UpdateForm(w http.ResponseWriter, r *http.Request) {
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

	// Extract form ID from URL
	formID := extractFormID(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
		return
	}

	// Parse and validate request
	var req models.UpdateFormRequest
	if err := h.validator.ValidateAndDecode(r, "form-update", &req); err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeErrorResponse(w, http.StatusBadRequest, "Validation error", valErr.Errors)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Update form
	form, err := h.formService.UpdateForm(r.Context(), formID, user.ID, req)
	if err != nil {
		log.Printf("action=update_form user_id=%s form_id=%s error=%q", user.ID, formID, err.Error())
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	log.Printf("action=update_form user_id=%s form_id=%s", user.ID, formID)
	writeJSONResponse(w, http.StatusOK, models.Response{Data: form})
}

// DeleteForm handles DELETE /forms/{id}
func (h *FormHandler) DeleteForm(w http.ResponseWriter, r *http.Request) {
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

	// Extract form ID from URL
	formID := extractFormID(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
		return
	}

	// Delete form
	if err := h.formService.DeleteForm(r.Context(), formID, user.ID); err != nil {
		log.Printf("action=delete_form user_id=%s form_id=%s error=%q", user.ID, formID, err.Error())
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete form")
		}
		return
	}

	log.Printf("action=delete_form user_id=%s form_id=%s", user.ID, formID)
	w.WriteHeader(http.StatusNoContent)
}

// GetFormStats handles GET /forms/{id}/stats
func (h *FormHandler) GetFormStats(w http.ResponseWriter, r *http.Request) {
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

	// Extract form ID from URL
	formID := extractFormID(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
		return
	}

	// Get stats
	stats, err := h.formService.GetFormStats(r.Context(), formID, user.ID)
	if err != nil {
		log.Printf("action=get_form_stats user_id=%s form_id=%s error=%q", user.ID, formID, err.Error())
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get form stats")
		}
		return
	}

	log.Printf("action=get_form_stats user_id=%s form_id=%s", user.ID, formID)
	writeJSONResponse(w, http.StatusOK, models.Response{Data: stats})
}

// GetFormSubmissions handles GET /forms/{id}/submissions
func (h *FormHandler) GetFormSubmissions(w http.ResponseWriter, r *http.Request) {
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

	// Extract form ID from URL
	formID := extractFormID(r.URL.Path)
	if formID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Form ID is required")
		return
	}

	// Parse pagination parameters
	opts := parsePaginationOptions(r)

	// Get submissions
	submissions, total, err := h.formService.GetFormSubmissions(r.Context(), formID, user.ID, opts)
	if err != nil {
		log.Printf("action=get_form_submissions user_id=%s form_id=%s error=%q", user.ID, formID, err.Error())
		if errors.Is(err, customErrors.ErrNotFound) || errors.Is(err, customErrors.ErrAccessDenied) {
			writeErrorResponse(w, http.StatusNotFound, "Form not found")
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get form submissions")
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

	log.Printf("action=get_form_submissions user_id=%s form_id=%s count=%d", user.ID, formID, len(submissions))
	writeJSONResponse(w, http.StatusOK, models.Response{Data: submissions, Meta: meta})
}

// GetFormsSummary handles GET /forms/summary
func (h *FormHandler) GetFormsSummary(w http.ResponseWriter, r *http.Request) {
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

	// Get forms summary
	summary, err := h.formService.GetFormsSummary(r.Context(), user.ID)
	if err != nil {
		log.Printf("action=get_forms_summary user_id=%s error=%q", user.ID, err.Error())
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get forms summary")
		return
	}

	log.Printf("action=get_forms_summary user_id=%s", user.ID)
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

	return models.PaginationOptions{
		Page:    page,
		PerPage: perPage,
	}
}

// extractFormID extracts form ID from URL path
func extractFormID(path string) string {
	// Trim the prefix and then split to get the ID
	// e.g., /forms/{id} or /forms/{id}/stats
	trimmedPath := strings.TrimPrefix(path, "/forms/")
	parts := strings.SplitN(trimmedPath, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
