package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const ConfigFileName = "/data/options.json"

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
func Load(args []string) (*Config, error) {
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

	var initFromFile = false

	if _, err := os.Stat(ConfigFileName); err == nil {
		jsonFile, err := os.Open(ConfigFileName)
		if err == nil {
			byteValue, _ := io.ReadAll(jsonFile)
			if err = json.Unmarshal(byteValue, &config); err == nil {
				initFromFile = true
			} else {
				fmt.Printf("error on unmarshal config from file %s\n", err.Error())
			}
		}
	}

	if !initFromFile {
		flags := flag.NewFlagSet(args[0], flag.ContinueOnError)

		flags.StringVar(&config.Server.Port, "port", lookupEnvOrString("PORT", config.Server.Port), "PORT")
		flags.DurationVar(&config.Server.ReadTimeout, "readTimeout", lookupEnvOrDuration("READ_TIMEOUT", config.Server.ReadTimeout), "READ_TIMEOUT")
		flags.DurationVar(&config.Server.WriteTimeout, "writeTimeout", lookupEnvOrDuration("WRITE_TIMEOUT", config.Server.WriteTimeout), "WRITE_TIMEOUT")

		var redisAddressesStr string
		flags.StringVar(&redisAddressesStr, "redisAddresses", lookupEnvOrString("REDIS_ADDRESSES", strings.Join(config.Redis.Addresses, ",")), "REDIS_ADDRESSES")

		flags.StringVar(&config.Redis.Password, "redisPassword", lookupEnvOrString("REDIS_PASSWORD", config.Redis.Password), "REDIS_PASSWORD")
		flags.IntVar(&config.Redis.DB, "redisDB", lookupEnvOrInt("REDIS_DB", config.Redis.DB), "REDIS_DB")
		flags.StringVar(&config.Redis.EmbeddedPort, "redisEmbeddedPort", lookupEnvOrString("REDKA_PORT", config.Redis.EmbeddedPort), "REDKA_PORT")
		flags.StringVar(&config.Redis.EmbeddedDBPath, "redisEmbeddedDBPath", lookupEnvOrString("REDKA_DB_PATH", config.Redis.EmbeddedDBPath), "REDKA_DB_PATH")
		flags.StringVar(&config.JWT.Secret, "jwtSecret", lookupEnvOrString("JWT_SECRET", config.JWT.Secret), "JWT_SECRET")
		flags.IntVar(&config.RateLimit.IPPerMinute, "rateLimitIPPerMinute", lookupEnvOrInt("RATE_LIMIT_IP_PER_MINUTE", config.RateLimit.IPPerMinute), "RATE_LIMIT_IP_PER_MINUTE")
		flags.IntVar(&config.RateLimit.GlobalPerMinute, "rateLimitGlobalPerMinute", lookupEnvOrInt("RATE_LIMIT_GLOBAL_PER_MINUTE", config.RateLimit.GlobalPerMinute), "RATE_LIMIT_GLOBAL_PER_MINUTE")
		flags.IntVar(&config.TTL.FreeDays, "ttlFreeDays", lookupEnvOrInt("TTL_FREE_DAYS", config.TTL.FreeDays), "TTL_FREE_DAYS")
		flags.IntVar(&config.TTL.ProDays, "ttlProDays", lookupEnvOrInt("TTL_PRO_DAYS", config.TTL.ProDays), "TTL_PRO_DAYS")

		if err := flags.Parse(args[1:]); err != nil {
			return config, fmt.Errorf("error parsing flags: %w", err)
		}

		// Преобразуем строку адресов Redis в слайс
		if redisAddressesStr != "" {
			config.Redis.Addresses = strings.Split(redisAddressesStr, ",")
		}
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
