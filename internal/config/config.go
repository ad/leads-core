package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig
	Redis     RedisConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
	TTL       TTLConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// RedisConfig holds Redis cluster configuration
type RedisConfig struct {
	Addresses      []string
	Password       string
	DB             int
	UseEmbedded    bool
	EmbeddedPort   string
	EmbeddedDBPath string
}

// JWTConfig holds JWT token validation configuration
type JWTConfig struct {
	Secret string
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	IPPerMinute     int
	GlobalPerMinute int
}

// TTLConfig holds TTL settings for different user plans
type TTLConfig struct {
	FreeDays int
	ProDays  int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		},
		Redis: RedisConfig{
			Addresses:      getEnvStringSlice("REDIS_ADDRESSES", []string{"localhost:6379"}),
			Password:       getEnv("REDIS_PASSWORD", ""),
			DB:             getEnvInt("REDIS_DB", 0),
			UseEmbedded:    false, // будет установлено ниже
			EmbeddedPort:   getEnv("REDKA_PORT", "6379"),
			EmbeddedDBPath: getEnv("REDKA_DB_PATH", "file:redka.db"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
		},
		RateLimit: RateLimitConfig{
			IPPerMinute:     getEnvInt("RATE_LIMIT_IP_PER_MINUTE", 1),
			GlobalPerMinute: getEnvInt("RATE_LIMIT_GLOBAL_PER_MINUTE", 1000),
		},
		TTL: TTLConfig{
			FreeDays: getEnvInt("TTL_FREE_DAYS", 30),
			ProDays:  getEnvInt("TTL_PRO_DAYS", 365),
		},
	}

	if config.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	// Определяем, нужно ли использовать встроенный Redis сервер
	if len(config.Redis.Addresses) > 0 && config.Redis.Addresses[0] == "redka" {
		config.Redis.UseEmbedded = true
	}

	return config, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvInt gets an environment variable as int with a default value
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

// getEnvDuration gets an environment variable as duration with a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}

// getEnvStringSlice gets an environment variable as string slice with a default value
func getEnvStringSlice(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return strings.Split(value, ",")
}
