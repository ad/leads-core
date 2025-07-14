package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ad/leads-core/internal/models"
)

// writeJSONResponse writes a JSON response
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, write a simple error message
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal server error"}`))
	}
}

// writeErrorResponse writes an error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string, details ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := models.ErrorResponse{
		Error: message,
	}

	if len(details) > 0 {
		errorResp.Details = details[0]
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// Fallback to simple string response if JSON encoding fails
		response := `{"error":"` + strings.ReplaceAll(message, `"`, `\"`) + `"}`
		w.Write([]byte(response))
	}
}
