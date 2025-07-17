package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/ad/leads-core/internal/auth"
	"github.com/ad/leads-core/internal/config"
	"github.com/ad/leads-core/internal/handlers"
	"github.com/ad/leads-core/internal/middleware"
	"github.com/ad/leads-core/internal/services"
	"github.com/ad/leads-core/internal/storage"
	"github.com/ad/leads-core/internal/validation"
	"github.com/ad/leads-core/pkg/logger"
	"github.com/ad/leads-core/pkg/metrics"
	"github.com/ad/leads-core/pkg/monitoring"
	"github.com/ad/leads-core/pkg/panel"
)

var version = "dev"
var GitCommit = "run"
var BuildDate = ""

func initVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			GitCommit = setting.Value
		case "vcs.time":
			BuildDate = setting.Value
		case "vcs.modified":
			modified = true
		}
	}
	if modified {
		GitCommit += "+CHANGES"
	}
}

func main() {
	// Initialize version information
	initVersion()

	// Initialize logging
	logger.Init("leads-core", fmt.Sprintf("%s-%s", version, GitCommit))

	// Initialize metrics
	metrics.Init()

	// Initialize alerts
	monitoring.InitAlerts()

	// Initialize middleware logging
	middleware.InitLogging()

	logger.Info("Starting Leads Core service")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Configuration loaded", map[string]interface{}{
		"server_port": cfg.Server.Port,
		"redis_addrs": cfg.Redis.Addresses,
	})

	// Initialize Redis client
	redisClient, err := storage.NewRedisClient(cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", map[string]interface{}{
			"error": err.Error(),
		})
	}
	defer redisClient.Close()

	// Wrap underlying Redis client with monitoring
	underlyingClient := redisClient.GetClient()
	redisMonitor := monitoring.NewRedisMonitor(underlyingClient)
	monitoredClient := redisMonitor.WrapClient()

	// Create a new RedisClient wrapper with the monitored client
	monitoredRedisClient := storage.NewRedisClientWithUniversal(monitoredClient)

	// Start Redis connection monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connectionMonitor := monitoring.NewConnectionMonitor(underlyingClient)
	go connectionMonitor.StartHealthCheck(ctx, 30*time.Second)

	// Start performance monitoring
	performanceMonitor := monitoring.NewPerformanceMonitor()
	go performanceMonitor.StartMetricsCollection(ctx, 15*time.Second)

	// Start system monitoring with alerts
	go monitoring.StartSystemMonitoring(ctx, 30*time.Second)

	// Start profiling server
	profileMonitor := monitoring.NewProfileMonitor()
	go func() {
		if err := profileMonitor.StartProfilingEndpoints(ctx, ":6060"); err != nil && err != http.ErrServerClosed {
			logger.Error("Profiling server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	logger.Info("Monitoring systems started", map[string]interface{}{
		"profiling_addr":        ":6060",
		"health_check_interval": "30s",
		"metrics_interval":      "15s",
		"alerts_interval":       "30s",
	})

	// Initialize repositories
	statsRepo := storage.NewRedisStatsRepository(monitoredRedisClient)
	widgetRepo := storage.NewRedisWidgetRepository(monitoredRedisClient, statsRepo)
	submissionRepo := storage.NewRedisSubmissionRepository(monitoredRedisClient)

	// Initialize services
	ttlConfig := services.TTLConfig{
		FreeDays: cfg.TTL.FreeDays,
		ProDays:  cfg.TTL.ProDays,
	}
	widgetService := services.NewWidgetService(widgetRepo, submissionRepo, statsRepo, ttlConfig)

	// Initialize export service
	exportService := services.NewExportService(submissionRepo, widgetRepo)

	// Initialize JWT validator
	jwtValidator := auth.NewJWTValidator(cfg.JWT.Secret)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtValidator)
	rateLimiter := middleware.NewRateLimiter(redisClient, cfg.RateLimit)

	// Initialize validator
	validator, err := validation.NewSchemaValidator()
	if err != nil {
		logger.Fatal("Failed to create validator", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Initialize handlers
	widgetHandler := handlers.NewWidgetHandler(widgetService, exportService, validator)
	publicHandler := handlers.NewPublicHandler(widgetService, validator)
	userHandler := handlers.NewUserHandler(widgetService, validator)
	healthHandler := handlers.NewHealthHandler(redisClient)

	// Panel handler
	panelHandler := panel.NewHandler()

	// Setup HTTP server with routes
	mux := http.NewServeMux()

	// System endpoints (no rate limiting)
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/metrics", metrics.Handler())

	// Admin panel (no authentication required as it handles auth internally)
	mux.Handle("/panel/", panelHandler)
	mux.Handle("/panel", panelHandler)

	// Public endpoints (with logging, metrics, and rate limiting)
	// These handle /widgets/{id}/submit and /widgets/{id}/events
	publicChain := middleware.LogRequests(metrics.HTTPMiddleware(rateLimiter.RateLimit(http.HandlerFunc(routePublicWidgetEndpoints(publicHandler)))))
	mux.Handle("/widgets/", publicChain)

	// Private API endpoints (with logging, metrics, and authentication only - no rate limiting)
	// API v1 endpoints for authenticated users
	privateWidgetsChain := middleware.LogRequests(metrics.HTTPMiddleware(authMiddleware.Authenticate(http.HandlerFunc(routePrivateWidgetEndpoints(widgetHandler)))))
	privateUsersChain := middleware.LogRequests(metrics.HTTPMiddleware(authMiddleware.Authenticate(http.HandlerFunc(routeUserEndpoints(userHandler)))))

	mux.Handle("/api/v1/widgets/", privateWidgetsChain)
	mux.Handle("/api/v1/widgets", privateWidgetsChain)
	mux.Handle("/api/v1/users/", privateUsersChain)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", map[string]interface{}{
			"port": cfg.Server.Port,
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Give the server 30 seconds to finish current requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Server exited gracefully")
}

// routePrivateWidgetEndpoints routes private widget endpoints for /api/v1/widgets/*
func routePrivateWidgetEndpoints(handler *handlers.WidgetHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Remove /api/v1/widgets prefix to get the actual path
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/widgets")

		switch {
		case path == "" || path == "/":
			// GET /api/v1/widgets - list widgets
			// POST /api/v1/widgets - create widget
			switch r.Method {
			case http.MethodGet:
				handler.GetWidgets(w, r)
			case http.MethodPost:
				handler.CreateWidget(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case path == "/summary":
			// GET /api/v1/widgets/summary
			if r.Method == http.MethodGet {
				handler.GetWidgetsSummary(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case strings.HasSuffix(path, "/stats"):
			// GET /api/v1/widgets/{id}/stats
			// Reconstruct URL as /widgets/{id}/stats for handler
			r.URL.Path = "/widgets" + path
			handler.GetWidgetStats(w, r)
		case strings.HasSuffix(path, "/submissions"):
			// GET /api/v1/widgets/{id}/submissions
			// Reconstruct URL as /widgets/{id}/submissions for handler
			r.URL.Path = "/widgets" + path
			handler.GetWidgetSubmissions(w, r)
		case strings.HasSuffix(path, "/export"):
			// GET /api/v1/widgets/{id}/export
			// Reconstruct URL as /widgets/{id}/export for handler
			r.URL.Path = "/widgets" + path
			handler.ExportWidgetSubmissions(w, r)
		default:
			// GET /api/v1/widgets/{id} - get widget
			// PUT /api/v1/widgets/{id} - update widget
			// DELETE /api/v1/widgets/{id} - delete widget
			// Reconstruct URL as /widgets/{id} for handler
			r.URL.Path = "/widgets" + path
			switch r.Method {
			case http.MethodGet:
				handler.GetWidget(w, r)
			case http.MethodPut:
				handler.UpdateWidget(w, r)
			case http.MethodDelete:
				handler.DeleteWidget(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	}
}

// routePublicWidgetEndpoints routes public widget endpoints
func routePublicWidgetEndpoints(handler *handlers.PublicHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/submit"):
			// POST /widgets/{id}/submit
			handler.SubmitWidget(w, r)
		case strings.HasSuffix(path, "/events"):
			// POST /widgets/{id}/events
			handler.RegisterEvent(w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}

// routeUserEndpoints routes user endpoints for /api/v1/users/*
func routeUserEndpoints(handler *handlers.UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/users")

		switch {
		case strings.HasSuffix(path, "/ttl"):
			// PUT /api/v1/users/{id}/ttl
			// Reconstruct URL as /users/{id}/ttl for handler
			r.URL.Path = "/users" + path
			if r.Method == http.MethodPut {
				handler.UpdateUserTTL(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}
