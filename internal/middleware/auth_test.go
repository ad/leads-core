package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/auth"
	"github.com/ad/leads-core/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

// TokenValidator interface for testing
type TokenValidator interface {
	ValidateToken(token string) (*models.User, error)
}

// MockJWTValidator implements TokenValidator for testing
type MockJWTValidator struct {
	users map[string]*models.User
	err   error
}

func NewMockJWTValidator() *MockJWTValidator {
	return &MockJWTValidator{
		users: make(map[string]*models.User),
	}
}

func (m *MockJWTValidator) AddValidUser(token string, user *models.User) {
	m.users[token] = user
}

func (m *MockJWTValidator) SetError(err error) {
	m.err = err
}

func (m *MockJWTValidator) ValidateToken(token string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	user, exists := m.users[token]
	if !exists {
		return nil, jwt.ErrSignatureInvalid
	}
	return user, nil
}

func TestAuthMiddleware_Integration(t *testing.T) {
	// Use real JWT validator for integration test
	secret := "test-secret-for-middleware"
	validator := auth.NewJWTValidator(secret)
	middleware := NewAuthMiddleware(validator, false)

	// Create a valid JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "test-user-123",
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Create a test handler
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Check if user was added to context
		contextUser, ok := auth.GetUserFromContext(r.Context())
		if !ok {
			t.Error("User not found")
		}
		if contextUser.ID != "test-user-123" {
			t.Errorf("Expected user ID test-user-123, got %s", contextUser.ID)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create request with valid token
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	// Execute middleware
	middleware.Authenticate(testHandler).ServeHTTP(rec, req)

	if !called {
		t.Error("Handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidTokenIntegration(t *testing.T) {
	secret := "test-secret-for-middleware"
	validator := auth.NewJWTValidator(secret)
	middleware := NewAuthMiddleware(validator, false)

	// Create a test handler
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Create request with invalid token
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	// Execute middleware
	middleware.Authenticate(testHandler).ServeHTTP(rec, req)

	if called {
		t.Error("Handler should not have been called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingTokenIntegration(t *testing.T) {
	secret := "test-secret-for-middleware"
	validator := auth.NewJWTValidator(secret)
	middleware := NewAuthMiddleware(validator, false)

	// Create a test handler
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Create request without Authorization header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Execute middleware
	middleware.Authenticate(testHandler).ServeHTTP(rec, req)

	if called {
		t.Error("Handler should not have been called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}
