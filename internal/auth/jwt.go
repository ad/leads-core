package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/ad/leads-core/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// UserContextKey is the context key for user data
	UserContextKey ContextKey = "user"
)

// JWTValidator handles JWT token validation
type JWTValidator struct {
	secret []byte
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(secret string) *JWTValidator {
	return &JWTValidator{
		secret: []byte(secret),
	}
}

// Claims represents JWT claims structure
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Plan     string `json:"plan,omitempty"`
	jwt.RegisteredClaims
}

// ValidateToken validates JWT token and returns user data
func (v *JWTValidator) ValidateToken(tokenString string) (*models.User, error) {
	// Remove "Bearer " prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate required fields
	if claims.UserID == "" {
		return nil, fmt.Errorf("user_id claim is required")
	}

	// Create user model
	user := &models.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Plan:     claims.Plan,
	}

	return user, nil
}

// GetUserFromContext extracts user from context
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok
}

// SetUserInContext adds user to context
func SetUserInContext(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}
