package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ad/leads-core/internal/models"
)

func TestE2E_SimpleFormCreation(t *testing.T) {
	e2e := setupE2EServer(t)
	userID := "test-user-simple"
	token := e2e.createTestToken(userID)
	headers := map[string]string{
		"Authorization": "Bearer " + token,
	}

	// Create a simple form
	createFormData := []byte(`{
		"name": "Simple Test Form",
		"type": "contact",
		"enabled": true,
		"fields": {
			"name": {"type": "text", "required": true}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/private/forms", createFormData, headers)
	if err != nil {
		t.Fatalf("Failed to create form: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
		// Read response body for debugging
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		t.Logf("Response body: %s", buf.String())
		return
	}

	var createdForm models.Form
	if err := json.NewDecoder(resp.Body).Decode(&createdForm); err != nil {
		t.Fatalf("Failed to decode form: %v", err)
	}

	if createdForm.ID == "" {
		t.Error("Form ID is empty")
	}
	if createdForm.Name != "Simple Test Form" {
		t.Errorf("Expected form name 'Simple Test Form', got %s", createdForm.Name)
	}
	if createdForm.OwnerID != userID {
		t.Errorf("Expected owner ID %s, got %s", userID, createdForm.OwnerID)
	}

	t.Logf("Successfully created form with ID: %s", createdForm.ID)
}

func TestE2E_SimpleHealthCheck(t *testing.T) {
	e2e := setupE2EServer(t)

	resp, err := e2e.makeRequest("GET", "/health", nil, nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", health["status"])
	}

	t.Log("Health check passed successfully")
}

func TestE2E_ComprehensiveFlow(t *testing.T) {
	e2e := setupE2EServer(t)
	userID := "comprehensive-test-user"
	token := e2e.createTestToken(userID)
	headers := map[string]string{
		"Authorization": "Bearer " + token,
	}

	// Step 1: Create multiple forms
	formData1 := []byte(`{
		"name": "Contact Form",
		"type": "contact",
		"enabled": true,
		"fields": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	formData2 := []byte(`{
		"name": "Newsletter Form",
		"type": "newsletter",
		"enabled": false,
		"fields": {
			"email": {"type": "email", "required": true}
		}
	}`)

	// Create first form
	resp1, err := e2e.makeRequest("POST", "/api/private/forms", formData1, headers)
	if err != nil {
		t.Fatalf("Failed to create first form: %v", err)
	}
	defer resp1.Body.Close()

	var form1 models.Form
	json.NewDecoder(resp1.Body).Decode(&form1)

	// Create second form
	resp2, err := e2e.makeRequest("POST", "/api/private/forms", formData2, headers)
	if err != nil {
		t.Fatalf("Failed to create second form: %v", err)
	}
	defer resp2.Body.Close()

	var form2 models.Form
	json.NewDecoder(resp2.Body).Decode(&form2)

	// Step 2: List forms - should return 2 forms
	listResp, err := e2e.makeRequest("GET", "/api/private/forms", nil, headers)
	if err != nil {
		t.Fatalf("Failed to list forms: %v", err)
	}
	defer listResp.Body.Close()

	var listResponse map[string]interface{}
	json.NewDecoder(listResp.Body).Decode(&listResponse)

	forms := listResponse["data"].([]interface{})
	if len(forms) != 2 {
		t.Errorf("Expected 2 forms, got %d", len(forms))
	}

	// Step 3: Submit to enabled form (should work)
	submissionData := []byte(`{
		"data": {
			"name": "Test User",
			"email": "test@example.com"
		}
	}`)

	submitResp, err := e2e.makeRequest("POST", "/api/forms/"+form1.ID+"/submit", submissionData, nil)
	if err != nil {
		t.Fatalf("Failed to submit to enabled form: %v", err)
	}
	defer submitResp.Body.Close()

	if submitResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 for enabled form submission, got %d", submitResp.StatusCode)
	}

	// Step 4: Try to submit to disabled form (should fail)
	submitResp2, err := e2e.makeRequest("POST", "/api/forms/"+form2.ID+"/submit", submissionData, nil)
	if err != nil {
		t.Fatalf("Failed to make request to disabled form: %v", err)
	}
	defer submitResp2.Body.Close()

	if submitResp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for disabled form submission, got %d", submitResp2.StatusCode)
	}

	// Step 5: Check stats for first form
	statsResp, err := e2e.makeRequest("GET", "/api/private/forms/"+form1.ID+"/stats", nil, headers)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	defer statsResp.Body.Close()

	if statsResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for stats, got %d", statsResp.StatusCode)
	}

	var stats map[string]interface{}
	json.NewDecoder(statsResp.Body).Decode(&stats)

	// Should have at least 1 submission and some views
	if stats["submits"] == "0" {
		t.Error("Expected at least 1 submission in stats")
	}

	// Step 6: Update form
	updateData := []byte(`{
		"name": "Updated Contact Form",
		"enabled": false
	}`)

	updateResp, err := e2e.makeRequest("PUT", "/api/private/forms/"+form1.ID, updateData, headers)
	if err != nil {
		t.Fatalf("Failed to update form: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for form update, got %d", updateResp.StatusCode)
	}

	// Step 7: Verify update worked
	getResp, err := e2e.makeRequest("GET", "/api/private/forms/"+form1.ID, nil, headers)
	if err != nil {
		t.Fatalf("Failed to get updated form: %v", err)
	}
	defer getResp.Body.Close()

	var updatedForm struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(getResp.Body).Decode(&updatedForm)

	formData := updatedForm.Data
	if formData["name"] != "Updated Contact Form" {
		t.Errorf("Form name not updated correctly: expected 'Updated Contact Form', got %v", formData["name"])
	}
	if formData["enabled"] != false {
		t.Errorf("Form enabled status not updated correctly: expected false, got %v", formData["enabled"])
	}

	t.Logf("Comprehensive flow test completed successfully")
}
