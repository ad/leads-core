package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/ad/leads-core/internal/auth"
)

// AuthMiddleware provides JWT authentication middleware
type AuthMiddleware struct {
	validator *auth.JWTValidator
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(validator *auth.JWTValidator) *AuthMiddleware {
	return &AuthMiddleware{
		validator: validator,
	}
}

// Authenticate validates JWT token and adds user to context
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeErrorResponse(w, http.StatusUnauthorized, "Authorization header is required")
			return
		}

		// Validate token
		user, err := m.validator.ValidateToken(authHeader)
		if err != nil {
			log.Printf("action=authenticate error=%q", err.Error())
			writeErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
			return
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
