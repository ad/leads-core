package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ad/leads-core/internal/errors"
	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/storage"
)

// FormService handles business logic for forms
type FormService struct {
	formRepo       storage.FormRepository
	submissionRepo storage.SubmissionRepository
	statsRepo      storage.StatsRepository
	config         TTLConfig
}

// TTLConfig holds TTL configuration
type TTLConfig struct {
	FreeDays int
	ProDays  int
}

// NewFormService creates a new form service
func NewFormService(
	formRepo storage.FormRepository,
	submissionRepo storage.SubmissionRepository,
	statsRepo storage.StatsRepository,
	ttlConfig TTLConfig,
) *FormService {
	return &FormService{
		formRepo:       formRepo,
		submissionRepo: submissionRepo,
		statsRepo:      statsRepo,
		config:         ttlConfig,
	}
}

// CreateForm creates a new form
func (s *FormService) CreateForm(ctx context.Context, userID string, req models.CreateFormRequest) (*models.Form, error) {
	// Validate request
	if req.Name == "" {
		return nil, fmt.Errorf("form name is required")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("form type is required")
	}

	// Generate ID (simple timestamp-based ID for now)
	formID := fmt.Sprintf("form_%d", time.Now().UnixNano())

	// Create form
	form := &models.Form{
		ID:        formID,
		OwnerID:   userID,
		Type:      req.Type,
		Name:      req.Name,
		Enabled:   req.Enabled,
		Fields:    req.Fields,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.formRepo.Create(ctx, form); err != nil {
		return nil, fmt.Errorf("failed to create form: %w", err)
	}

	return form, nil
}

// GetForm retrieves a form by ID with ownership check
func (s *FormService) GetForm(ctx context.Context, formID, userID string) (*models.Form, error) {
	form, err := s.formRepo.GetByID(ctx, formID)
	if err != nil {
		return nil, errors.ErrNotFound
	}

	// Check ownership
	if form.OwnerID != userID {
		return nil, errors.ErrAccessDenied
	}

	return form, nil
}

// UpdateForm updates an existing form
func (s *FormService) UpdateForm(ctx context.Context, formID, userID string, req models.UpdateFormRequest) (*models.Form, error) {
	// Get existing form
	form, err := s.GetForm(ctx, formID, userID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != nil {
		form.Name = *req.Name
	}
	if req.Type != nil {
		form.Type = *req.Type
	}
	if req.Enabled != nil {
		form.Enabled = *req.Enabled
	}
	if req.Fields != nil {
		form.Fields = req.Fields
	}

	form.UpdatedAt = time.Now()

	if err := s.formRepo.Update(ctx, form); err != nil {
		return nil, fmt.Errorf("failed to update form: %w", err)
	}

	return form, nil
}

// DeleteForm deletes a form
func (s *FormService) DeleteForm(ctx context.Context, formID, userID string) error {
	// Check ownership first
	_, err := s.GetForm(ctx, formID, userID)
	if err != nil {
		return err
	}

	if err := s.formRepo.Delete(ctx, formID); err != nil {
		return fmt.Errorf("failed to delete form: %w", err)
	}

	return nil
}

// GetUserForms retrieves forms for a user with pagination
func (s *FormService) GetUserForms(ctx context.Context, userID string, opts models.PaginationOptions) ([]*models.Form, int, error) {
	forms, total, err := s.formRepo.GetByUserID(ctx, userID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user forms: %w", err)
	}

	return forms, total, nil
}

// GetFormStats retrieves statistics for a form
func (s *FormService) GetFormStats(ctx context.Context, formID, userID string) (*models.FormStats, error) {
	// Check ownership
	_, err := s.GetForm(ctx, formID, userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.statsRepo.GetFormStats(ctx, formID)
	if err != nil {
		return nil, fmt.Errorf("failed to get form stats: %w", err)
	}

	return stats, nil
}

// GetFormSubmissions retrieves submissions for a form
func (s *FormService) GetFormSubmissions(ctx context.Context, formID, userID string, opts models.PaginationOptions) ([]*models.Submission, int, error) {
	// Check ownership
	_, err := s.GetForm(ctx, formID, userID)
	if err != nil {
		return nil, 0, err
	}

	submissions, total, err := s.submissionRepo.GetByFormID(ctx, formID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get form submissions: %w", err)
	}

	return submissions, total, nil
}

// SubmitForm submits data to a form (public endpoint)
func (s *FormService) SubmitForm(ctx context.Context, formID string, req models.SubmissionRequest) (*models.Submission, error) {
	// Get form (no ownership check for public endpoint)
	form, err := s.formRepo.GetByID(ctx, formID)
	if err != nil {
		return nil, errors.ErrNotFound
	}

	// Check if form is enabled
	if !form.Enabled {
		return nil, errors.ErrFormDisabled
	}

	// Generate submission ID
	submissionID := fmt.Sprintf("sub_%d", time.Now().UnixNano())

	// Calculate TTL based on form owner's plan (this would come from user data)
	// For now, use default TTL
	ttl := time.Duration(s.config.FreeDays) * 24 * time.Hour

	// Create submission
	submission := &models.Submission{
		ID:        submissionID,
		FormID:    formID,
		Data:      req.Data,
		CreatedAt: time.Now(),
		TTL:       ttl,
	}

	if err := s.submissionRepo.Create(ctx, submission); err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	// Increment submit count
	if err := s.statsRepo.IncrementSubmits(ctx, formID); err != nil {
		// Log error but don't fail the submission
		// log.Printf("failed to increment submit count for form %s: %v", formID, err)
	}

	return submission, nil
}

// RegisterFormEvent registers a form event (view, close)
func (s *FormService) RegisterFormEvent(ctx context.Context, formID string, eventType string) error {
	// Check if form exists and is enabled
	form, err := s.formRepo.GetByID(ctx, formID)
	if err != nil {
		return fmt.Errorf("form not found: %w", err)
	}

	if !form.Enabled {
		return fmt.Errorf("form is disabled")
	}

	// Register event
	switch eventType {
	case "view":
		if err := s.statsRepo.IncrementViews(ctx, formID); err != nil {
			return fmt.Errorf("failed to register view event: %w", err)
		}
	case "close":
		if err := s.statsRepo.IncrementCloses(ctx, formID); err != nil {
			return fmt.Errorf("failed to register close event: %w", err)
		}
	default:
		return fmt.Errorf("unknown event type: %s", eventType)
	}

	return nil
}

// UpdateUserTTL updates TTL for all submissions of a user
func (s *FormService) UpdateUserTTL(ctx context.Context, userID string, plan string) error {
	var newTTL time.Duration
	switch plan {
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
func (s *FormService) UpdateUserSubmissionsTTL(ctx context.Context, userID string, ttlDays int) error {
	const perPage = 100 // Process in batches of 100
	page := 1

	for {
		// Get a page of user's forms
		forms, _, err := s.formRepo.GetByUserID(ctx, userID, models.PaginationOptions{
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return fmt.Errorf("failed to get user forms on page %d: %w", page, err)
		}

		// Update TTL for submissions of each form
		for _, form := range forms {
			if err := s.submissionRepo.UpdateFormSubmissionsTTL(ctx, form.ID, ttlDays); err != nil {
				log.Printf("Failed to update TTL for form %s submissions: %v", form.ID, err)
				// Continue with other forms
			}
		}

		// If we received fewer forms than we asked for, it's the last page
		if len(forms) < perPage {
			break
		}

		page++
	}

	return nil
}

// GetFormsSummary returns a summary of user's forms
func (s *FormService) GetFormsSummary(ctx context.Context, userID string) (*models.FormsSummary, error) {
	summary := &models.FormsSummary{}
	const perPage = 100 // Process in batches of 100
	page := 1

	for {
		forms, total, err := s.formRepo.GetByUserID(ctx, userID, models.PaginationOptions{
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get user forms for summary on page %d: %w", page, err)
		}

		if page == 1 {
			summary.TotalForms = total
		}

		// Count active/disabled forms and calculate totals
		for _, form := range forms {
			if form.Enabled {
				summary.ActiveForms++
			} else {
				summary.DisabledForms++
			}

			// Get form stats
			stats, err := s.statsRepo.GetFormStats(ctx, form.ID)
			if err != nil {
				// Log error but continue
				log.Printf("Failed to get stats for form %s: %v", form.ID, err)
				continue
			}

			summary.TotalViews += int(stats.Views)
			summary.TotalSubmissions += int(stats.Submits)
		}

		// If we received fewer forms than we asked for, it's the last page
		if len(forms) < perPage {
			break
		}

		page++
	}

	return summary, nil
}
