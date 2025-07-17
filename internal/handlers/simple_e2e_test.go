package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ad/leads-core/internal/models"
)

func TestE2E_SimpleWidgetCreation(t *testing.T) {
	e2e := setupE2EServer(t)
	userID := "test-user-simple"
	token := e2e.createTestToken(userID)
	headers := map[string]string{
		"Authorization": "Bearer " + token,
	}

	// Create a simple widget
	createWidgetData := []byte(`{
		"name": "Simple Test Widget",
		"type": "lead-form",
		"enabled": true,
		"description": "Simple test widget for E2E testing",
		"fields": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	resp, err := e2e.makeRequest("POST", "/api/v1/widgets", createWidgetData, headers)
	if err != nil {
		t.Fatalf("Failed to create widget: %v", err)
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

	var response models.Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Extract widget from response data
	widgetData, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Widget data is not a map")
	}

	widgetID, ok := widgetData["id"].(string)
	if !ok || widgetID == "" {
		t.Fatal("Widget ID is empty")
	}

	if name, ok := widgetData["name"].(string); !ok || name != "Simple Test Widget" {
		t.Errorf("Expected widget name 'Simple Test Widget', got %v", name)
	}

	// Note: user_id may not be included in the response for security reasons

	t.Logf("Successfully created widget with ID: %s", widgetID)
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

	// Step 1: Create multiple widgets
	widgetData1 := []byte(`{
		"name": "Contact Widget",
		"type": "lead-form",
		"enabled": true,
		"fields": {
			"name": {"type": "text", "required": true},
			"email": {"type": "email", "required": true}
		}
	}`)

	widgetData2 := []byte(`{
		"name": "Banner Widget",
		"type": "banner",
		"enabled": false,
		"fields": {
			"email": {"type": "email", "required": true}
		}
	}`)

	// Create first widget
	resp1, err := e2e.makeRequest("POST", "/api/v1/widgets", widgetData1, headers)
	if err != nil {
		t.Fatalf("Failed to create first widget: %v", err)
	}
	defer resp1.Body.Close()

	var response1 models.Response
	json.NewDecoder(resp1.Body).Decode(&response1)
	widget1Data := response1.Data.(map[string]interface{})
	widget1ID := widget1Data["id"].(string)

	// Create second widget
	resp2, err := e2e.makeRequest("POST", "/api/v1/widgets", widgetData2, headers)
	if err != nil {
		t.Fatalf("Failed to create second widget: %v", err)
	}
	defer resp2.Body.Close()

	var response2 models.Response
	json.NewDecoder(resp2.Body).Decode(&response2)
	widget2Data := response2.Data.(map[string]interface{})
	widget2ID := widget2Data["id"].(string)

	// Step 2: List widgets - should return 2 widgets
	listResp, err := e2e.makeRequest("GET", "/api/v1/widgets", nil, headers)
	if err != nil {
		t.Fatalf("Failed to list widgets: %v", err)
	}
	defer listResp.Body.Close()

	var listResponse map[string]interface{}
	json.NewDecoder(listResp.Body).Decode(&listResponse)

	widgets := listResponse["data"].([]interface{})
	if len(widgets) != 2 {
		t.Errorf("Expected 2 widgets, got %d", len(widgets))
	}

	// Step 3: Submit to enabled widget (should work)
	submissionData := []byte(`{
		"data": {
			"name": "Test User",
			"email": "test@example.com"
		}
	}`)

	submitResp, err := e2e.makeRequest("POST", "/widgets/"+widget1ID+"/submit", submissionData, nil)
	if err != nil {
		t.Fatalf("Failed to submit to enabled widget: %v", err)
	}
	defer submitResp.Body.Close()

	if submitResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 for enabled widget submission, got %d", submitResp.StatusCode)
	}

	// Step 4: Try to submit to disabled widget (should fail)
	submitResp2, err := e2e.makeRequest("POST", "/widgets/"+widget2ID+"/submit", submissionData, nil)
	if err != nil {
		t.Fatalf("Failed to make request to disabled widget: %v", err)
	}
	defer submitResp2.Body.Close()

	if submitResp2.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for disabled widget submission, got %d", submitResp2.StatusCode)
	}

	// Step 5: Check stats for first widget
	statsResp, err := e2e.makeRequest("GET", "/api/v1/widgets/"+widget1ID+"/stats", nil, headers)
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

	// Step 6: Update widget
	updateData := []byte(`{
		"name": "Updated Contact Widget",
		"enabled": false
	}`)

	updateResp, err := e2e.makeRequest("PUT", "/api/v1/widgets/"+widget1ID, updateData, headers)
	if err != nil {
		t.Fatalf("Failed to update widget: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for widget update, got %d", updateResp.StatusCode)
	}

	// Step 7: Verify update worked
	getResp, err := e2e.makeRequest("GET", "/api/v1/widgets/"+widget1ID, nil, headers)
	if err != nil {
		t.Fatalf("Failed to get updated widget: %v", err)
	}
	defer getResp.Body.Close()

	var updatedWidget struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(getResp.Body).Decode(&updatedWidget)

	widgetData := updatedWidget.Data
	if widgetData["name"] != "Updated Contact Widget" {
		t.Errorf("Widget name not updated correctly: expected 'Updated Contact Widget', got %v", widgetData["name"])
	}
	if widgetData["enabled"] != false {
		t.Errorf("Widget enabled status not updated correctly: expected false, got %v", widgetData["enabled"])
	}

	t.Logf("Comprehensive flow test completed successfully")
}
