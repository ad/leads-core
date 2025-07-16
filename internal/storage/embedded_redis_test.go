package storage

import (
	"context"
	"testing"
	"time"

	"github.com/ad/leads-core/internal/config"
)

func TestEmbeddedRedis(t *testing.T) {
	cfg := config.RedisConfig{
		Addresses:      []string{"redka"},
		UseEmbedded:    true,
		EmbeddedPort:   "6381",
		EmbeddedDBPath: ":memory:",
	}

	redisClient, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create Redis client: %v", err)
	}
	defer redisClient.Close()

	ctx := context.Background()
	client := redisClient.GetClient()

	err = client.Set(ctx, "test_key", "test_value", 5*time.Minute).Err()
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	val, err := client.Get(ctx, "test_key").Result()
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if val != "test_value" {
		t.Fatalf("Expected 'test_value', got %s", val)
	}

	t.Log("✅ Embedded Redis test passed!")
}
