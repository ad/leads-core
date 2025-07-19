package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ad/leads-core/internal/auth"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/validation"
	"github.com/ad/leads-core/pkg/logger"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	widgetService *services.WidgetService
	validator     *validation.SchemaValidator
}

// NewUserHandler creates a new user handler
func NewUserHandler(widgetService *services.WidgetService, validator *validation.SchemaValidator) *UserHandler {
	return &UserHandler{
		widgetService: widgetService,
		validator:     validator,
	}
}

// GetUser handles GET /api/v1/user - returns current user information
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
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

	logger.Debug("Retrieved user information", map[string]interface{}{
		"action":  "get_user",
		"user_id": user.ID,
		"plan":    user.Plan,
	})

	// Return user information
	writeJSONResponse(w, http.StatusOK, user)
}

// UpdateUserTTL handles PUT /users/{id}/ttl
func (h *UserHandler) UpdateUserTTL(w http.ResponseWriter, r *http.Request) {
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

	// Extract user ID from URL
	userID := extractUserIDFromTTLPath(r.URL.Path)
	if userID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "User ID is required")
		return
	}

	if userID == "demo" {
		writeErrorResponse(w, http.StatusForbidden, "Cannot update TTL for demo user")
		return
	}

	// Check if user can update TTL for this user (only self or admin)
	if user.ID != userID {
		writeErrorResponse(w, http.StatusForbidden, "Cannot update TTL for other users")
		return
	}

	// Parse request
	var req models.UpdateTTLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate TTL days
	if req.TTLDays <= 0 || req.TTLDays > 3650 { // Max 10 years
		writeErrorResponse(w, http.StatusBadRequest, "TTL days must be between 1 and 3650")
		return
	}

	// Update TTL for user's submissions
	err := h.widgetService.UpdateUserSubmissionsTTL(r.Context(), userID, req.TTLDays)
	if err != nil {
		logger.Error("Failed to update user TTL", map[string]interface{}{
			"action":   "update_user_ttl",
			"user_id":  userID,
			"ttl_days": req.TTLDays,
			"error":    err.Error(),
		})
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to update TTL")
		return
	}

	logger.Debug("Updated user TTL successfully", map[string]interface{}{
		"action":   "update_user_ttl",
		"user_id":  userID,
		"ttl_days": req.TTLDays,
	})
	writeJSONResponse(w, http.StatusOK, models.Response{
		Data: map[string]interface{}{
			"message":  "TTL updated successfully",
			"user_id":  userID,
			"ttl_days": req.TTLDays,
		},
	})
}

// extractUserIDFromTTLPath extracts user ID from paths like /users/{id}/ttl
func extractUserIDFromTTLPath(path string) string {
	// Remove leading/trailing slashes and split
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expected format: ["users", "{id}", "ttl"]
	if len(parts) == 3 && parts[0] == "users" && parts[2] == "ttl" {
		return parts[1]
	}
	return ""
}
