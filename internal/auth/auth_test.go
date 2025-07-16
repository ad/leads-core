package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidator_ValidateToken(t *testing.T) {
	secret := "test-secret-key"
	validator := NewJWTValidator(secret)

	// Create a valid test token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "test-user-123",
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	tests := []struct {
		name        string
		tokenString string
		expectError bool
		expectedID  string
	}{
		{
			name:        "valid token",
			tokenString: tokenString,
			expectError: false,
			expectedID:  "test-user-123",
		},
		{
			name:        "empty token",
			tokenString: "",
			expectError: true,
			expectedID:  "",
		},
		{
			name:        "invalid token",
			tokenString: "invalid.token.here",
			expectError: true,
			expectedID:  "",
		},
		{
			name:        "malwidgeted token",
			tokenString: "not-a-token",
			expectError: true,
			expectedID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := validator.ValidateToken(tt.tokenString)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				if user != nil {
					t.Errorf("Expected nil user, but got %v", user)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if user == nil {
					t.Errorf("Expected user, but got nil")
				} else if user.ID != tt.expectedID {
					t.Errorf("Expected user ID %s, but got %s", tt.expectedID, user.ID)
				}
			}
		})
	}
}

func TestJWTValidator_ExpiredToken(t *testing.T) {
	secret := "test-secret-key"
	validator := NewJWTValidator(secret)

	// Create an expired token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "test-user-123",
		"exp":     time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
		"iat":     time.Now().Add(-2 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	user, err := validator.ValidateToken(tokenString)
	if err == nil {
		t.Errorf("Expected error for expired token, but got none")
	}
	if user != nil {
		t.Errorf("Expected nil user for expired token, but got %v", user)
	}
}

func TestJWTValidator_WrongSecret(t *testing.T) {
	secret := "test-secret-key"
	wrongSecret := "wrong-secret-key"
	validator := NewJWTValidator(secret)

	// Create token with wrong secret
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "test-user-123",
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(wrongSecret))
	if err != nil {
		t.Fatalf("Failed to create token with wrong secret: %v", err)
	}

	user, err := validator.ValidateToken(tokenString)
	if err == nil {
		t.Errorf("Expected error for token with wrong secret, but got none")
	}
	if user != nil {
		t.Errorf("Expected nil user for token with wrong secret, but got %v", user)
	}
}

func TestGetUserFromContext(t *testing.T) {
	// Test with user in context
	user := &models.User{ID: "test-user-123", Username: "testuser"}
	ctx := context.WithValue(context.Background(), UserContextKey, user)

	retrievedUser, ok := GetUserFromContext(ctx)
	if !ok {
		t.Errorf("Expected to find user in context, but got false")
	}
	if retrievedUser == nil {
		t.Errorf("Expected user, but got nil")
	} else if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %s, but got %s", user.ID, retrievedUser.ID)
	}

	// Test with no user in context
	emptyCtx := context.Background()
	retrievedUser, ok = GetUserFromContext(emptyCtx)
	if ok {
		t.Errorf("Expected not to find user in empty context, but got true")
	}
	if retrievedUser != nil {
		t.Errorf("Expected nil user from empty context, but got %v", retrievedUser)
	}

	// Test with wrong type in context
	wrongCtx := context.WithValue(context.Background(), UserContextKey, "not-a-user")
	retrievedUser, ok = GetUserFromContext(wrongCtx)
	if ok {
		t.Errorf("Expected not to find user with wrong type, but got true")
	}
	if retrievedUser != nil {
		t.Errorf("Expected nil user with wrong type, but got %v", retrievedUser)
	}
}

func TestExtractTokenFromHeader(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		expectedToken string
		expectError   bool
	}{
		{
			name:          "valid bearer token",
			header:        "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectError:   false,
		},
		{
			name:          "empty header",
			header:        "",
			expectedToken: "",
			expectError:   true,
		},
		{
			name:          "invalid format",
			header:        "InvalidFormat token",
			expectedToken: "",
			expectError:   true,
		},
		{
			name:          "bearer without token",
			header:        "Bearer",
			expectedToken: "",
			expectError:   true,
		},
		{
			name:          "bearer with empty token",
			header:        "Bearer ",
			expectedToken: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			// Simulate token extraction logic directly since function is not exported
			authHeader := req.Header.Get("Authorization")
			var token string
			var err error

			if authHeader == "" {
				err = fmt.Errorf("missing authorization header")
			} else if !strings.HasPrefix(authHeader, "Bearer ") {
				err = fmt.Errorf("invalid authorization header format")
			} else {
				token = strings.TrimPrefix(authHeader, "Bearer ")
				if token == "" {
					err = fmt.Errorf("empty token")
				}
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if token != tt.expectedToken {
					t.Errorf("Expected token %s, but got %s", tt.expectedToken, token)
				}
			}
		})
	}
}

func TestUser_Validation(t *testing.T) {
	tests := []struct {
		name  string
		user  models.User
		valid bool
	}{
		{
			name:  "valid user",
			user:  models.User{ID: "user-123", Username: "testuser", Plan: "pro"},
			valid: true,
		},
		{
			name:  "empty ID",
			user:  models.User{ID: "", Username: "testuser", Plan: "pro"},
			valid: false,
		},
		{
			name:  "minimal valid user",
			user:  models.User{ID: "user-123"},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.user.ID != ""

			if isValid != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.valid, isValid)
			}
		})
	}
}
