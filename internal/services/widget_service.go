package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ad/leads-core/internal/errors"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/storage"
	"github.com/ad/leads-core/pkg/logger"
	"github.com/google/uuid"
)

// WidgetService handles business logic for widgets
type WidgetService struct {
	widgetRepo     storage.WidgetRepository
	submissionRepo storage.SubmissionRepository
	statsRepo      storage.StatsRepository
	config         TTLConfig
}

// TTLConfig holds TTL configuration
type TTLConfig struct {
	DemoDays int
	FreeDays int
	ProDays  int
}

// NewWidgetService creates a new widget service
func NewWidgetService(
	widgetRepo storage.WidgetRepository,
	submissionRepo storage.SubmissionRepository,
	statsRepo storage.StatsRepository,
	ttlConfig TTLConfig,
) *WidgetService {
	return &WidgetService{
		widgetRepo:     widgetRepo,
		submissionRepo: submissionRepo,
		statsRepo:      statsRepo,
		config:         ttlConfig,
	}
}

// generateWidgetID generates a UUID v5 using user_id as namespace
func (s *WidgetService) generateWidgetID(userID string) string {
	// Create a namespace UUID from user_id
	userNamespace := uuid.NewSHA1(uuid.NameSpaceOID, []byte(userID))

	// Use timestamp for uniqueness within user namespace
	name := fmt.Sprintf("widget_%d", time.Now().UnixNano())

	// Generate UUID v5
	widgetUUID := uuid.NewSHA1(userNamespace, []byte(name))

	return widgetUUID.String()
}

// generateSubmissionID generates a UUID v5 using widget_id as namespace
func (s *WidgetService) generateSubmissionID(widgetID string) string {
	// Create a namespace UUID from widget_id
	widgetNamespace := uuid.NewSHA1(uuid.NameSpaceOID, []byte(widgetID))

	// Use timestamp for uniqueness within widget namespace
	name := fmt.Sprintf("submission_%d", time.Now().UnixNano())

	// Generate UUID v5
	submissionUUID := uuid.NewSHA1(widgetNamespace, []byte(name))

	return submissionUUID.String()
}

// CreateWidget creates a new widget
func (s *WidgetService) CreateWidget(ctx context.Context, userID string, req models.CreateWidgetRequest) (*models.Widget, error) {
	// Validate request
	if req.Name == "" {
		return nil, fmt.Errorf("widget name is required")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("widget type is required")
	}

	// Generate UUID v5 using user_id as namespace
	widgetID := s.generateWidgetID(userID)

	// Create widget
	widget := &models.Widget{
		ID:        widgetID,
		OwnerID:   userID,
		Type:      req.Type,
		Name:      req.Name,
		IsVisible: req.IsVisible,
		Config:    req.Config,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.widgetRepo.Create(ctx, widget); err != nil {
		return nil, fmt.Errorf("failed to create widget: %w", err)
	}

	return widget, nil
}

// GetWidget retrieves a widget by ID with ownership check
func (s *WidgetService) GetWidget(ctx context.Context, widgetID, userID string) (*models.Widget, error) {
	widget, err := s.widgetRepo.GetByID(ctx, widgetID)
	if err != nil {
		return nil, errors.ErrNotFound
	}

	// Check ownership
	if widget.OwnerID != userID {
		return nil, errors.ErrAccessDenied
	}

	return widget, nil
}

// UpdateWidget updates an existing widget
func (s *WidgetService) UpdateWidget(ctx context.Context, widgetID, userID string, req models.UpdateWidgetRequest) (*models.Widget, error) {
	// Get existing widget
	widget, err := s.GetWidget(ctx, widgetID, userID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != nil {
		widget.Name = *req.Name
	}
	if req.Type != nil {
		widget.Type = *req.Type
	}
	if req.IsVisible != nil {
		widget.IsVisible = *req.IsVisible
	}

	widget.UpdatedAt = time.Now()

	if err := s.widgetRepo.Update(ctx, widget); err != nil {
		return nil, fmt.Errorf("failed to update widget: %w", err)
	}

	return widget, nil
}

// UpdateWidgetConfig updates the configuration of a widget
func (s *WidgetService) UpdateWidgetConfig(ctx context.Context, widgetID, userID string, req *models.UpdateWidgetConfigRequest) (*models.Widget, error) {
	// Check ownership first
	widget, err := s.GetWidget(ctx, widgetID, userID)
	if err != nil {
		return nil, err
	}

	// Update config
	widget.Config = req.Config
	widget.UpdatedAt = time.Now()

	if err := s.widgetRepo.Update(ctx, widget); err != nil {
		return nil, fmt.Errorf("failed to update widget config: %w", err)
	}

	return widget, nil
}

// DeleteWidget deletes a widget
func (s *WidgetService) DeleteWidget(ctx context.Context, widgetID, userID string) error {
	// Check ownership first
	_, err := s.GetWidget(ctx, widgetID, userID)
	if err != nil {
		return err
	}

	if err := s.widgetRepo.Delete(ctx, widgetID); err != nil {
		return fmt.Errorf("failed to delete widget: %w", err)
	}

	return nil
}

// GetUserWidgets retrieves widgets for a user with pagination
func (s *WidgetService) GetUserWidgets(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Widget, int, error) {
	widgets, total, err := s.widgetRepo.GetByUserID(ctx, userID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user widgets: %w", err)
	}

	return widgets, total, nil
}

// GetWidgetStats retrieves statistics for a widget
func (s *WidgetService) GetWidgetStats(ctx context.Context, widgetID, userID string) (*models.WidgetStats, error) {
	// Check ownership
	_, err := s.GetWidget(ctx, widgetID, userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.statsRepo.GetWidgetStats(ctx, widgetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get widget stats: %w", err)
	}

	return stats, nil
}

// GetWidgetSubmissions retrieves submissions for a widget
func (s *WidgetService) GetWidgetSubmissions(ctx context.Context, widgetID, userID string, opts models.PaginationOptions) ([]*models.Submission, int, error) {
	// Check ownership
	_, err := s.GetWidget(ctx, widgetID, userID)
	if err != nil {
		return nil, 0, err
	}

	submissions, total, err := s.submissionRepo.GetByWidgetID(ctx, widgetID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get widget submissions: %w", err)
	}

	return submissions, total, nil
}

// SubmitWidget submits data to a widget (public endpoint)
func (s *WidgetService) SubmitWidget(ctx context.Context, widgetID string, req models.SubmissionRequest) (*models.Submission, error) {
	// Get widget (no ownership check for public endpoint)
	widget, err := s.widgetRepo.GetByID(ctx, widgetID)
	if err != nil {
		return nil, errors.ErrNotFound
	}

	// Check if widget is enabled
	if !widget.IsVisible {
		return nil, errors.ErrWidgetDisabled
	}

	// Generate submission ID using UUID v5
	submissionID := s.generateSubmissionID(widgetID)

	// Calculate TTL based on widget owner's plan (this would come from user data)
	// For now, use default TTL
	ttl := time.Duration(s.config.FreeDays) * 24 * time.Hour

	// Create submission
	submission := &models.Submission{
		ID:        submissionID,
		WidgetID:  widgetID,
		Data:      req.Data,
		CreatedAt: time.Now(),
		TTL:       ttl,
	}

	if err := s.submissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	// Increment submit count
	if err := s.statsRepo.IncrementSubmits(ctx, widgetID); err != nil {
		// Log error but don't fail the submission
		logger.Error("failed to increment submit count for widget", map[string]interface{}{
			"widget_id": widgetID,
			"error":     err,
		})
	}

	return submission, nil
}

// RegisterWidgetEvent registers a widget event (view, close)
func (s *WidgetService) RegisterWidgetEvent(ctx context.Context, widgetID string, eventType string) error {
	// Check if widget exists and is enabled
	widget, err := s.widgetRepo.GetByID(ctx, widgetID)
	if err != nil {
		return fmt.Errorf("widget not found: %w", err)
	}

	if !widget.IsVisible {
		return fmt.Errorf("widget is disabled")
	}

	// Register event
	switch eventType {
	case "view":
		if err := s.statsRepo.IncrementViews(ctx, widgetID); err != nil {
			return fmt.Errorf("failed to register view event: %w", err)
		}
	case "close":
		if err := s.statsRepo.IncrementCloses(ctx, widgetID); err != nil {
			return fmt.Errorf("failed to register close event: %w", err)
		}
	default:
		return fmt.Errorf("unknown event type: %s", eventType)
	}

	return nil
}

// UpdateUserTTL updates TTL for all submissions of a user
func (s *WidgetService) UpdateUserTTL(ctx context.Context, userID string, plan string) error {
	var newTTL time.Duration
	switch plan {
	case "demo":
		newTTL = time.Duration(s.config.DemoDays) * 24 * time.Hour
	case "pro":
		newTTL = time.Duration(s.config.ProDays) * 24 * time.Hour
	default:
		newTTL = time.Duration(s.config.FreeDays) * 24 * time.Hour
	}

	if err := s.submissionRepo.UpdateTTL(ctx, userID, newTTL); err != nil {
		return fmt.Errorf("failed to update user TTL: %w", err)
	}

	return nil
}

// UpdateUserSubmissionsTTL updates TTL for all submissions of a user
func (s *WidgetService) UpdateUserSubmissionsTTL(ctx context.Context, userID string, ttlDays int) error {
	const perPage = 100 // Process in batches of 100
	page := 1

	for {
		// Get a page of user's widgets
		widgets, _, err := s.widgetRepo.GetByUserID(ctx, userID, models.PaginationOptions{
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return fmt.Errorf("failed to get user widgets on page %d: %w", page, err)
		}

		// Update TTL for submissions of each widget
		for _, widget := range widgets {
			if err := s.submissionRepo.UpdateWidgetSubmissionsTTL(ctx, widget.ID, ttlDays); err != nil {
				logger.Error("Failed to update TTL for widget submissions", map[string]interface{}{
					"action":    "update_widget_ttl",
					"widget_id": widget.ID,
					"ttl_days":  ttlDays,
					"error":     err.Error(),
				})
				// Continue with other widgets
			}
		}

		// If we received fewer widgets than we asked for, it's the last page
		if len(widgets) < perPage {
			break
		}

		page++
	}

	return nil
}

// GetWidgetsSummary returns a summary of user's widgets
func (s *WidgetService) GetWidgetsSummary(ctx context.Context, userID string) (*models.WidgetsSummary, error) {
	summary := &models.WidgetsSummary{}
	const perPage = 100 // Process in batches of 100
	page := 1

	for {
		widgets, total, err := s.widgetRepo.GetByUserID(ctx, userID, models.PaginationOptions{
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get user widgets for summary on page %d: %w", page, err)
		}

		if page == 1 {
			summary.TotalWidgets = total
		}

		// Count active/disabled widgets and calculate totals
		for _, widget := range widgets {
			if widget.IsVisible {
				summary.ActiveWidgets++
			} else {
				summary.DisabledWidgets++
			}

			// Get widget stats
			stats, err := s.statsRepo.GetWidgetStats(ctx, widget.ID)
			if err != nil {
				// Log error but continue
				logger.Error("Failed to get stats for widget", map[string]interface{}{
					"action":    "get_widget_stats_for_summary",
					"widget_id": widget.ID,
					"error":     err.Error(),
				})
				continue
			}

			summary.TotalViews += int(stats.Views)
			summary.TotalSubmissions += int(stats.Submits)
		}

		// If we received fewer widgets than we asked for, it's the last page
		if len(widgets) < perPage {
			break
		}

		page++
	}

	return summary, nil
}
