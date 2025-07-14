package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity level of log messages
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger represents a structured logger
type Logger struct {
	output   io.Writer
	level    LogLevel
	service  string
	version  string
	hostname string
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Service   string                 `json:"service"`
	Version   string                 `json:"version,omitempty"`
	Hostname  string                 `json:"hostname,omitempty"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	File      string                 `json:"file,omitempty"`
	Function  string                 `json:"function,omitempty"`
}

// New creates a new structured logger
func New(service, version string) *Logger {
	hostname, _ := os.Hostname()

	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	level := INFO // default
	switch levelStr {
	case "DEBUG":
		level = DEBUG
	case "INFO":
		level = INFO
	case "WARN":
		level = WARN
	case "ERROR":
		level = ERROR
	case "FATAL":
		level = FATAL
	}

	return &Logger{
		output:   os.Stdout,
		level:    level,
		service:  service,
		version:  version,
		hostname: hostname,
	}
}

// SetOutput sets the output destination for the logger
func (l *Logger) SetOutput(w io.Writer) {
	l.output = w
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// log writes a log entry if the level is appropriate
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Service:   l.service,
		Version:   l.version,
		Hostname:  l.hostname,
		Message:   message,
		Fields:    fields,
	}

	// Add caller information for non-production environments
	if level >= ERROR || l.level == DEBUG {
		if pc, file, line, ok := runtime.Caller(2); ok {
			entry.File = fmt.Sprintf("%s:%d", file, line)
			if fn := runtime.FuncForPC(pc); fn != nil {
				entry.Function = fn.Name()
			}
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	fmt.Fprintf(l.output, "%s\n", data)

	// Exit for FATAL level
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message with optional fields
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

// Info logs an info message with optional fields
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

// Warn logs a warning message with optional fields
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

// Error logs an error message with optional fields
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}

// Fatal logs a fatal message with optional fields and exits
func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, message, f)
}

// WithFields returns a FieldLogger with predefined fields
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// FieldLogger allows logging with predefined fields
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// mergeFields combines predefined fields with new ones
func (fl *FieldLogger) mergeFields(newFields map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for k, v := range fl.fields {
		merged[k] = v
	}
	for k, v := range newFields {
		merged[k] = v
	}
	return merged
}

// Debug logs a debug message with merged fields
func (fl *FieldLogger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(DEBUG, message, f)
}

// Info logs an info message with merged fields
func (fl *FieldLogger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(INFO, message, f)
}

// Warn logs a warning message with merged fields
func (fl *FieldLogger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(WARN, message, f)
}

// Error logs an error message with merged fields
func (fl *FieldLogger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(ERROR, message, f)
}

// Fatal logs a fatal message with merged fields and exits
func (fl *FieldLogger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(FATAL, message, f)
}

// Global logger instance
var defaultLogger *Logger

// Init initializes the global logger
func Init(service, version string) {
	defaultLogger = New(service, version)
}

// Global logging functions
func Debug(message string, fields ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debug(message, fields...)
	}
}

func Info(message string, fields ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(message, fields...)
	}
}

func Warn(message string, fields ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warn(message, fields...)
	}
}

func Error(message string, fields ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(message, fields...)
	}
}

func Fatal(message string, fields ...map[string]interface{}) {
	if defaultLogger != nil {
		defaultLogger.Fatal(message, fields...)
	}
}

func WithFields(fields map[string]interface{}) *FieldLogger {
	if defaultLogger != nil {
		return defaultLogger.WithFields(fields)
	}
	return nil
}
