package main

import (
	"fmt"
	"log"

	_ "github.com/joho/godotenv/autoload"

	"github.com/ad/leads-core/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Println("Configuration loaded successfully:")
	fmt.Printf("Server Port: %s\n", cfg.Server.Port)
	fmt.Printf("Redis Addresses: %v\n", cfg.Redis.Addresses)
	fmt.Printf("JWT Secret: %s\n", maskSecret(cfg.JWT.Secret))
	fmt.Printf("Rate Limit IP: %d/min\n", cfg.RateLimit.IPPerMinute)
	fmt.Printf("Rate Limit Global: %d/min\n", cfg.RateLimit.GlobalPerMinute)
	fmt.Printf("TTL Free Days: %d\n", cfg.TTL.FreeDays)
	fmt.Printf("TTL Pro Days: %d\n", cfg.TTL.ProDays)
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}
