package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestLogger_Basic(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create logger with buffer output
	logger := New("test-service", "1.0.0")
	logger.SetOutput(&buf)
	logger.SetLevel(DEBUG)

	// Test info logging
	logger.Info("test message", map[string]interface{}{
		"key": "value",
	})

	// Parse the logged JSON
	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Verify log entry
	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", entry.Message)
	}
	if entry.Service != "test-service" {
		t.Errorf("Expected service 'test-service', got %s", entry.Service)
	}
	if entry.Fields["key"] != "value" {
		t.Errorf("Expected field key=value, got %v", entry.Fields["key"])
	}
}

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test", "1.0.0")
	logger.SetOutput(&buf)

	tests := []struct {
		level    LogLevel
		logFunc  func(string, ...map[string]interface{})
		expected string
	}{
		{DEBUG, logger.Debug, "DEBUG"},
		{INFO, logger.Info, "INFO"},
		{WARN, logger.Warn, "WARN"},
		{ERROR, logger.Error, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			buf.Reset()
			logger.SetLevel(tt.level)

			tt.logFunc("test message")

			if buf.Len() == 0 {
				t.Errorf("Expected log output, got none")
				return
			}

			var entry LogEntry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("Failed to parse log entry: %v", err)
			}

			if entry.Level != tt.expected {
				t.Errorf("Expected level %s, got %s", tt.expected, entry.Level)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test", "1.0.0")
	logger.SetOutput(&buf)
	logger.SetLevel(WARN) // Only WARN and above should be logged

	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() != 0 {
		t.Errorf("Expected no output for DEBUG/INFO when level is WARN, got: %s", buf.String())
	}

	logger.Warn("warn message")

	if buf.Len() == 0 {
		t.Errorf("Expected output for WARN message")
	}
}

func TestFieldLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test", "1.0.0")
	logger.SetOutput(&buf)

	fieldLogger := logger.WithFields(map[string]interface{}{
		"component": "test_component",
		"user_id":   "123",
	})

	fieldLogger.Info("test message", map[string]interface{}{
		"action": "create",
	})

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Check that both predefined and additional fields are present
	if entry.Fields["component"] != "test_component" {
		t.Errorf("Expected component field")
	}
	if entry.Fields["user_id"] != "123" {
		t.Errorf("Expected user_id field")
	}
	if entry.Fields["action"] != "create" {
		t.Errorf("Expected action field")
	}
}

func TestGlobalLogger(t *testing.T) {
	// Initialize global logger
	Init("global-test", "1.0.0")

	// Test global functions - these should not panic
	Info("global info message")
	Debug("global debug message")
	Warn("global warn message")
	Error("global error message")

	// Test with fields
	WithFields(map[string]interface{}{
		"global": true,
	}).Info("global field message")
}

func TestLogLevelsFromEnv(t *testing.T) {
	tests := []struct {
		envValue string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"INFO", INFO},
		{"WARN", WARN},
		{"ERROR", ERROR},
		{"FATAL", FATAL},
		{"invalid", INFO}, // default
		{"", INFO},        // default
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			// Set environment variable
			os.Setenv("LOG_LEVEL", tt.envValue)
			defer os.Unsetenv("LOG_LEVEL")

			logger := New("test", "1.0.0")

			// Since we can't directly access the level field,
			// we test by checking if messages are logged
			var buf bytes.Buffer
			logger.SetOutput(&buf)

			logger.Debug("debug test")

			// If level is DEBUG, we should see output
			// If level is higher, we shouldn't
			hasOutput := buf.Len() > 0
			shouldHaveOutput := tt.expected == DEBUG

			if hasOutput != shouldHaveOutput {
				t.Errorf("Level %s: expected output=%t, got output=%t",
					tt.envValue, shouldHaveOutput, hasOutput)
			}
		})
	}
}
