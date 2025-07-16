package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/ad/leads-core/pkg/logger"
	"github.com/ad/leads-core/pkg/metrics"
)

// AlertLevel represents the severity of an alert
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert represents a system alert
type Alert struct {
	ID         string                 `json:"id"`
	Level      AlertLevel             `json:"level"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Resolved   bool                   `json:"resolved"`
	ResolvedAt *time.Time             `json:"resolved_at,omitempty"`
}

// AlertManager manages system alerts
type AlertManager struct {
	mu       sync.RWMutex
	alerts   map[string]*Alert
	handlers []AlertHandler
	logger   *logger.FieldLogger
}

// AlertHandler defines the interface for alert handlers
type AlertHandler interface {
	HandleAlert(alert *Alert) error
}

// LogAlertHandler logs alerts using structured logging
type LogAlertHandler struct {
	logger *logger.FieldLogger
}

// NewLogAlertHandler creates a new log alert handler
func NewLogAlertHandler() *LogAlertHandler {
	return &LogAlertHandler{
		logger: logger.WithFields(map[string]interface{}{
			"component": "alert_handler",
		}),
	}
}

// HandleAlert logs the alert
func (h *LogAlertHandler) HandleAlert(alert *Alert) error {
	fields := map[string]interface{}{
		"alert_id":    alert.ID,
		"alert_level": string(alert.Level),
		"title":       alert.Title,
		"timestamp":   alert.Timestamp,
	}

	// Add metadata to fields
	for k, v := range alert.Metadata {
		fields["meta_"+k] = v
	}

	switch alert.Level {
	case AlertLevelCritical:
		h.logger.Error(alert.Message, fields)
	case AlertLevelWarning:
		h.logger.Warn(alert.Message, fields)
	default:
		h.logger.Info(alert.Message, fields)
	}

	return nil
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts: make(map[string]*Alert),
		logger: logger.WithFields(map[string]interface{}{
			"component": "alert_manager",
		}),
	}
}

// AddHandler adds an alert handler
func (am *AlertManager) AddHandler(handler AlertHandler) {
	am.handlers = append(am.handlers, handler)
}

// TriggerAlert triggers a new alert
func (am *AlertManager) TriggerAlert(id, title, message string, level AlertLevel, metadata map[string]interface{}) {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert := &Alert{
		ID:        id,
		Level:     level,
		Title:     title,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  metadata,
		Resolved:  false,
	}

	am.alerts[id] = alert

	// Update metrics
	labels := map[string]string{
		"level": string(level),
	}
	metrics.Inc("alerts_triggered_total", labels, "Total alerts triggered")

	// Send to handlers
	for _, handler := range am.handlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(alert); err != nil {
				am.logger.Error("Failed to handle alert", map[string]interface{}{
					"alert_id": id,
					"error":    err.Error(),
				})
			}
		}(handler)
	}

	am.logger.Info("Alert triggered", map[string]interface{}{
		"alert_id": id,
		"level":    string(level),
		"title":    title,
	})
}

// ResolveAlert resolves an existing alert
func (am *AlertManager) ResolveAlert(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, exists := am.alerts[id]; exists && !alert.Resolved {
		now := time.Now()
		alert.Resolved = true
		alert.ResolvedAt = &now

		// Update metrics
		labels := map[string]string{
			"level": string(alert.Level),
		}
		metrics.Inc("alerts_resolved_total", labels, "Total alerts resolved")

		am.logger.Info("Alert resolved", map[string]interface{}{
			"alert_id": id,
			"duration": now.Sub(alert.Timestamp).String(),
		})
	}
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var active []*Alert
	for _, alert := range am.alerts {
		if !alert.Resolved {
			active = append(active, alert)
		}
	}
	return active
}

// GetAllAlerts returns all alerts
func (am *AlertManager) GetAllAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var all []*Alert
	for _, alert := range am.alerts {
		all = append(all, alert)
	}
	return all
}

// SystemMonitor monitors system health and triggers alerts
type SystemMonitor struct {
	alertManager *AlertManager
	logger       *logger.FieldLogger
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(alertManager *AlertManager) *SystemMonitor {
	return &SystemMonitor{
		alertManager: alertManager,
		logger: logger.WithFields(map[string]interface{}{
			"component": "system_monitor",
		}),
	}
}

// StartMonitoring starts system health monitoring
func (sm *SystemMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sm.logger.Info("Starting system health monitoring", map[string]interface{}{
		"interval": interval.String(),
	})

	for {
		select {
		case <-ctx.Done():
			sm.logger.Info("System monitoring stopped")
			return
		case <-ticker.C:
			sm.checkSystemHealth()
		}
	}
}

// checkSystemHealth perwidgets system health checks
func (sm *SystemMonitor) checkSystemHealth() {
	// Check Redis connectivity (from metrics)
	allMetrics := metrics.GetMetrics()
	if allMetrics != nil {
		if redisUp, exists := allMetrics["redis_connection_up"]; exists {
			if redisUp.Value == 0 {
				sm.alertManager.TriggerAlert(
					"redis_connection_down",
					"Redis Connection Down",
					"Redis connection is not available",
					AlertLevelCritical,
					map[string]interface{}{
						"metric": "redis_connection_up",
						"value":  redisUp.Value,
					},
				)
			} else {
				sm.alertManager.ResolveAlert("redis_connection_down")
			}
		}

		// Check high error rate
		if errorsTotal, exists := allMetrics["http_responses_total{status=500}"]; exists {
			if errorsTotal.Value > 10 { // More than 10 500 errors
				sm.alertManager.TriggerAlert(
					"high_error_rate",
					"High HTTP Error Rate",
					"High number of HTTP 500 errors detected",
					AlertLevelWarning,
					map[string]interface{}{
						"error_count": errorsTotal.Value,
						"threshold":   10,
					},
				)
			}
		}

		// Check memory usage
		if memAlloc, exists := allMetrics["system_memory_alloc_bytes"]; exists {
			memAllocMB := memAlloc.Value / 1024 / 1024
			if memAllocMB > 500 { // More than 500MB
				sm.alertManager.TriggerAlert(
					"high_memory_usage",
					"High Memory Usage",
					"Application memory usage is high",
					AlertLevelWarning,
					map[string]interface{}{
						"memory_mb": memAllocMB,
						"threshold": 500,
					},
				)
			} else {
				sm.alertManager.ResolveAlert("high_memory_usage")
			}
		}
	}
}

// Global alert manager
var defaultAlertManager *AlertManager

// InitAlerts initializes the global alert manager
func InitAlerts() {
	defaultAlertManager = NewAlertManager()
	defaultAlertManager.AddHandler(NewLogAlertHandler())
}

// TriggerAlert triggers an alert using the global alert manager
func TriggerAlert(id, title, message string, level AlertLevel, metadata map[string]interface{}) {
	if defaultAlertManager != nil {
		defaultAlertManager.TriggerAlert(id, title, message, level, metadata)
	}
}

// ResolveAlert resolves an alert using the global alert manager
func ResolveAlert(id string) {
	if defaultAlertManager != nil {
		defaultAlertManager.ResolveAlert(id)
	}
}

// GetActiveAlerts returns active alerts from the global alert manager
func GetActiveAlerts() []*Alert {
	if defaultAlertManager != nil {
		return defaultAlertManager.GetActiveAlerts()
	}
	return nil
}

// StartSystemMonitoring starts system monitoring with the global alert manager
func StartSystemMonitoring(ctx context.Context, interval time.Duration) {
	if defaultAlertManager != nil {
		monitor := NewSystemMonitor(defaultAlertManager)
		go monitor.StartMonitoring(ctx, interval)
	}
}
