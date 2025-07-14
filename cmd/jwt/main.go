package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	var (
		secret = flag.String("secret", "", "JWT secret key")
		userID = flag.String("user", "", "User ID")
		ttl    = flag.Duration("ttl", 24*time.Hour, "Token TTL (default: 24h)")
	)
	flag.Parse()

	if *secret == "" {
		fmt.Fprintf(os.Stderr, "Error: secret is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -secret=<jwt-secret> -user=<user-id> [-ttl=<duration>]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s -secret=my-secret -user=user123 -ttl=1h\n", os.Args[0])
		os.Exit(1)
	}

	if *userID == "" {
		fmt.Fprintf(os.Stderr, "Error: user ID is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -secret=<jwt-secret> -user=<user-id> [-ttl=<duration>]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s -secret=my-secret -user=user123 -ttl=1h\n", os.Args[0])
		os.Exit(1)
	}

	// Create the Claims
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": *userID,
		"iat":     now.Unix(),
		"exp":     now.Add(*ttl).Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token
	tokenString, err := token.SignedString([]byte(*secret))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating token: %v\n", err)
		os.Exit(1)
	}

	// Output the token
	fmt.Println(tokenString)

	// Optional: Show token info in verbose mode
	if os.Getenv("VERBOSE") == "1" {
		fmt.Fprintf(os.Stderr, "Token generated successfully:\n")
		fmt.Fprintf(os.Stderr, "  User ID: %s\n", *userID)
		fmt.Fprintf(os.Stderr, "  Issued At: %s\n", now.Format(time.RFC3339))
		fmt.Fprintf(os.Stderr, "  Expires At: %s\n", now.Add(*ttl).Format(time.RFC3339))
		fmt.Fprintf(os.Stderr, "  TTL: %s\n", *ttl)
	}
}
