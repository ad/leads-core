package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/ad/leads-core/internal/models"
	"github.com/ad/leads-core/internal/storage"
	"github.com/ad/leads-core/pkg/logger"
	"github.com/xuri/excelize/v2"
)

// ExportService handles exporting submissions in various formats
type ExportService struct {
	submissionRepo storage.SubmissionRepository
	widgetRepo     storage.WidgetRepository
}

// NewExportService creates a new export service
func NewExportService(
	submissionRepo storage.SubmissionRepository,
	widgetRepo storage.WidgetRepository,
) *ExportService {
	return &ExportService{
		submissionRepo: submissionRepo,
		widgetRepo:     widgetRepo,
	}
}

// ExportSubmissions exports submissions for a widget in the specified format
func (s *ExportService) ExportSubmissions(ctx context.Context, widgetID, userID string, options models.ExportOptions) ([]byte, string, error) {
	// Verify widget ownership
	widget, err := s.widgetRepo.GetByID(ctx, widgetID)
	if err != nil {
		logger.Error("Failed to get widget for export", map[string]interface{}{
			"action":    "export_submissions",
			"widget_id": widgetID,
			"user_id":   userID,
			"error":     err.Error(),
		})
		return nil, "", fmt.Errorf("widget not found")
	}

	if widget.OwnerID != userID {
		logger.Warn("Unauthorized export attempt", map[string]interface{}{
			"action":    "export_submissions",
			"widget_id": widgetID,
			"user_id":   userID,
			"owner_id":  widget.OwnerID,
		})
		return nil, "", fmt.Errorf("unauthorized")
	}

	// Get all submissions for the widget with time filter
	submissions, err := s.getFilteredSubmissions(ctx, widgetID, options)
	if err != nil {
		logger.Error("Failed to get submissions for export", map[string]interface{}{
			"action":    "export_submissions",
			"widget_id": widgetID,
			"user_id":   userID,
			"error":     err.Error(),
		})
		return nil, "", err
	}

	var data []byte
	var filename string

	switch options.Format {
	case "csv":
		data, err = s.exportToCSV(submissions, widget)
		filename = fmt.Sprintf("%s_submissions_%s.csv", widget.Name, time.Now().Format("2006-01-02"))
	case "json":
		data, err = s.exportToJSON(submissions, widget)
		filename = fmt.Sprintf("%s_submissions_%s.json", widget.Name, time.Now().Format("2006-01-02"))
	case "xlsx":
		data, err = s.exportToXLSX(submissions, widget)
		filename = fmt.Sprintf("%s_submissions_%s.xlsx", widget.Name, time.Now().Format("2006-01-02"))
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", options.Format)
	}

	if err != nil {
		logger.Error("Failed to export submissions", map[string]interface{}{
			"action":    "export_submissions",
			"widget_id": widgetID,
			"user_id":   userID,
			"format":    options.Format,
			"error":     err.Error(),
		})
		return nil, "", err
	}

	logger.Info("Submissions exported successfully", map[string]interface{}{
		"action":    "export_submissions",
		"widget_id": widgetID,
		"user_id":   userID,
		"format":    options.Format,
		"count":     len(submissions),
		"filename":  filename,
	})

	return data, filename, nil
}

// getFilteredSubmissions retrieves submissions with optional time filtering
func (s *ExportService) getFilteredSubmissions(ctx context.Context, widgetID string, options models.ExportOptions) ([]*models.Submission, error) {
	// Get all submissions using pagination with large limit
	allSubmissions, _, err := s.submissionRepo.GetByWidgetID(ctx, widgetID, models.PaginationOptions{
		Page:    1,
		PerPage: 10000, // Large number to get all submissions
	})
	if err != nil {
		return nil, err
	}

	if options.From == nil && options.To == nil {
		return allSubmissions, nil
	}

	var filtered []*models.Submission
	for _, submission := range allSubmissions {
		include := true

		if options.From != nil && submission.CreatedAt.Before(*options.From) {
			include = false
		}

		if options.To != nil && submission.CreatedAt.After(*options.To) {
			include = false
		}

		if include {
			filtered = append(filtered, submission)
		}
	}

	return filtered, nil
}

// exportToCSV exports submissions to CSV format
func (s *ExportService) exportToCSV(submissions []*models.Submission, widget *models.Widget) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	if len(submissions) == 0 {
		// Write header only
		header := []string{"ID", "Created At"}
		writer.Write(header)
		writer.Flush()
		return buf.Bytes(), nil
	}

	// Collect all possible field names from all submissions
	fieldNames := s.collectFieldNames(submissions)

	// Write header
	header := []string{"ID", "Created At"}
	header = append(header, fieldNames...)
	writer.Write(header)

	// Write data rows
	for _, submission := range submissions {
		row := []string{
			submission.ID,
			submission.CreatedAt.Format(time.RFC3339),
		}

		// Add field values in the same order as header
		for _, fieldName := range fieldNames {
			value := ""
			if val, exists := submission.Data[fieldName]; exists {
				value = s.formatValue(val)
			}
			row = append(row, value)
		}

		writer.Write(row)
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// exportToJSON exports submissions to JSON format
func (s *ExportService) exportToJSON(submissions []*models.Submission, widget *models.Widget) ([]byte, error) {
	exportData := map[string]interface{}{
		"widget": map[string]interface{}{
			"id":   widget.ID,
			"name": widget.Name,
			"type": widget.Type,
		},
		"exported_at": time.Now().Format(time.RFC3339),
		"total_count": len(submissions),
		"submissions": submissions,
	}

	return json.MarshalIndent(exportData, "", "  ")
}

// exportToXLSX exports submissions to Excel format
func (s *ExportService) exportToXLSX(submissions []*models.Submission, widget *models.Widget) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Submissions"

	// Rename default sheet
	f.SetSheetName("Sheet1", sheetName)

	if len(submissions) == 0 {
		// Write header only
		f.SetCellValue(sheetName, "A1", "ID")
		f.SetCellValue(sheetName, "B1", "Created At")

		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Collect all possible field names
	fieldNames := s.collectFieldNames(submissions)

	// Write header
	f.SetCellValue(sheetName, "A1", "ID")
	f.SetCellValue(sheetName, "B1", "Created At")

	for i, fieldName := range fieldNames {
		col := s.numberToColumnName(i + 3) // Start from column C
		f.SetCellValue(sheetName, col+"1", fieldName)
	}

	// Style header row
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
	})

	headerRange := fmt.Sprintf("A1:%s1", s.numberToColumnName(len(fieldNames)+2))
	f.SetCellStyle(sheetName, "A1", headerRange, headerStyle)

	// Write data rows
	for i, submission := range submissions {
		rowNum := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), submission.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), submission.CreatedAt.Format(time.RFC3339))

		for j, fieldName := range fieldNames {
			col := s.numberToColumnName(j + 3)
			value := ""
			if val, exists := submission.Data[fieldName]; exists {
				value = s.formatValue(val)
			}
			f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, rowNum), value)
		}
	}

	// Auto-fit columns
	for i := 0; i < len(fieldNames)+2; i++ {
		col := s.numberToColumnName(i + 1)
		f.SetColWidth(sheetName, col, col, 15)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// collectFieldNames collects all unique field names from submissions
func (s *ExportService) collectFieldNames(submissions []*models.Submission) []string {
	fieldSet := make(map[string]bool)
	var fieldNames []string

	for _, submission := range submissions {
		for fieldName := range submission.Data {
			if !fieldSet[fieldName] {
				fieldSet[fieldName] = true
				fieldNames = append(fieldNames, fieldName)
			}
		}
	}

	return fieldNames
}

// formatValue converts interface{} to string for export
func (s *ExportService) formatValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return strconv.FormatBool(v)
	case []interface{}, map[string]interface{}:
		// For complex types, marshal to JSON
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", v)
	default:
		// Use reflection for other types
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			// Handle slices/arrays
			if jsonBytes, err := json.Marshal(v); err == nil {
				return string(jsonBytes)
			}
		}
		return fmt.Sprintf("%v", v)
	}
}

// numberToColumnName converts number to Excel column name (1=A, 2=B, 27=AA, etc.)
func (s *ExportService) numberToColumnName(num int) string {
	var result string
	for num > 0 {
		num--
		result = string(rune('A'+num%26)) + result
		num /= 26
	}
	return result
}
