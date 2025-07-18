package middleware

import (
	"net/http"
	"strings"

	"github.com/ad/leads-core/internal/auth"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/pkg/logger"
)

// AuthMiddleware provides JWT authentication middleware
type AuthMiddleware struct {
	validator *auth.JWTValidator
	allowDemo bool
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(validator *auth.JWTValidator, allowDemo bool) *AuthMiddleware {
	return &AuthMiddleware{
		validator: validator,
		allowDemo: allowDemo,
	}
}

// Authenticate validates JWT token and adds user to context
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")

		var user *models.User
		var err error

		if authHeader == "" {
			// If no auth header and demo mode is enabled, create demo user
			if m.allowDemo {
				user = &models.User{
					ID:       "demo",
					Username: "demo",
					Plan:     "demo",
					// Add other demo user fields as needed
				}
				logger.Debug("Using demo user", map[string]interface{}{
					"action": "authenticate",
					"user":   "demo",
				})
			} else {
				writeErrorResponse(w, http.StatusUnauthorized, "Authorization header is required")
				return
			}
		} else {
			// Validate token
			user, err = m.validator.ValidateToken(authHeader)
			if err != nil {
				logger.Debug("Authentication failed", map[string]interface{}{
					"action": "authenticate",
					"error":  err.Error(),
				})
				writeErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
		}

		// Add user to context
		ctx := auth.SetUserInContext(r.Context(), user)
		r = r.WithContext(ctx)

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// RequireAuth is a convenience method that combines authentication with authorization check
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return m.Authenticate(next)
}

// writeErrorResponse writes an error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Simple JSON response without importing encoding/json to avoid circular imports
	response := `{"error":"` + strings.ReplaceAll(message, `"`, `\"`) + `"}`
	w.Write([]byte(response))
}
