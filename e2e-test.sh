#!/bin/bash

# Full E2E Test Suite for Leads Core Service
# Tests all endpoints, authentication, rate limiting, and functionality

# set -e  # Exit on any error (commented out for debugging)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
SERVER_URL="http://localhost:8080"
JWT_SECRET="super-secret-jwt-key-change-in-production"
TEST_USER="e2e-test-user"

# Global test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
    ((PASSED_TESTS++))
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
    ((FAILED_TESTS++))
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_status="$3"
    
    ((TOTAL_TESTS++))
    log_info "Testing: $test_name"
    
    local response
    local status_code
    
    response=$(eval "$test_command" 2>&1)
    status_code=$?
    
    if [ "$status_code" = "$expected_status" ]; then
        log_success "$test_name"
        echo "$response" | head -3
    else
        log_error "$test_name (Exit code: $status_code, Expected: $expected_status)"
        echo "$response" | head -3
    fi
    echo
}

test_http() {
    local test_name="$1"
    local method="$2"
    local url="$3"
    local headers="$4"
    local data="$5"
    local expected_code="$6"
    
    ((TOTAL_TESTS++))
    log_info "Testing: $test_name"
    
    local curl_cmd="curl -s -w '\\nHTTP_CODE:%{http_code}\\n'"
    
    if [ -n "$headers" ]; then
        curl_cmd="$curl_cmd $headers"
    fi
    
    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -d '$data'"
    fi
    
    curl_cmd="$curl_cmd -X $method '$url'"
    
    local response
    response=$(eval "$curl_cmd")
    local http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    local body=$(echo "$response" | grep -v "HTTP_CODE:")
    
    if [ "$http_code" = "$expected_code" ]; then
        log_success "$test_name (HTTP $http_code)"
        if [ -n "$body" ] && command -v jq >/dev/null 2>&1 && echo "$body" | jq empty 2>/dev/null; then
            echo "$body" | jq -C . | head -10
        else
            echo "$body" | head -3
        fi
    else
        log_error "$test_name (Expected HTTP $expected_code, got $http_code)"
        echo "$body" | head -3
    fi
    echo
}

echo "🚀 Leads Core Service - Full E2E Test Suite"
echo "==========================================="
echo

# Generate test JWT token
log_info "Setting up test environment..."
if [ ! -f "./bin/jwt-gen" ]; then
    log_error "JWT generator not found. Please build the project first: make build"
    exit 1
fi

TOKEN=$(./bin/jwt-gen -secret="$JWT_SECRET" -user="$TEST_USER" -ttl="1h")
AUTH_HEADER="-H 'Authorization: Bearer $TOKEN'"
log_success "JWT token generated for user: $TEST_USER"
echo

# Test 1: System Health Endpoints
echo "📊 SYSTEM HEALTH TESTS"
echo "====================="

test_http "Health Check" "GET" "$SERVER_URL/health" "" "" "200"
test_http "Metrics Endpoint" "GET" "$SERVER_URL/metrics" "" "" "200"

# Test 2: Authentication Tests
echo "🔐 AUTHENTICATION TESTS"
echo "======================"

test_http "Access private endpoint without token" "GET" "$SERVER_URL/api/v1/forms" "" "" "401"
test_http "Access private endpoint with invalid token" "GET" "$SERVER_URL/api/v1/forms" "-H 'Authorization: Bearer invalid-token'" "" "401"
test_http "Access private endpoint with valid token" "GET" "$SERVER_URL/api/v1/forms" "$AUTH_HEADER" "" "200"

# Test 3: Private Form Management Endpoints (Authenticated)
echo "📝 PRIVATE FORM MANAGEMENT TESTS"
echo "==============================="

# Create a form and extract its ID
FORM_DATA='{"name":"Test Form","type":"contact","enabled":true,"fields":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}'
log_info "Creating form and extracting ID..."
response=$(curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$FORM_DATA" "$SERVER_URL/api/v1/forms")
CREATED_FORM_ID=$(echo "$response" | jq -r '.data.id // empty')

if [ -n "$CREATED_FORM_ID" ] && [ "$CREATED_FORM_ID" != "null" ]; then
    log_success "Form created with ID: $CREATED_FORM_ID"
    test_http "Create Form" "POST" "$SERVER_URL/api/v1/forms" "$AUTH_HEADER -H 'Content-Type: application/json'" "$FORM_DATA" "201"
    
    # Now test operations with the real form ID
    # Note: Form might be disabled by default, so some tests may return 403/404
    test_http "Get Real Form" "GET" "$SERVER_URL/api/v1/forms/$CREATED_FORM_ID" "$AUTH_HEADER" "" "200"
    
    # Update the real form
    UPDATE_DATA='{"name":"Updated Test Form","type":"contact"}'
    test_http "Update Real Form" "PUT" "$SERVER_URL/api/v1/forms/$CREATED_FORM_ID" "$AUTH_HEADER -H 'Content-Type: application/json'" "$UPDATE_DATA" "200"
    
    # Get stats for real form
    test_http "Get Real Form Stats" "GET" "$SERVER_URL/api/v1/forms/$CREATED_FORM_ID/stats" "$AUTH_HEADER" "" "200"
    
    # Get submissions for real form
    test_http "Get Real Form Submissions" "GET" "$SERVER_URL/api/v1/forms/$CREATED_FORM_ID/submissions" "$AUTH_HEADER" "" "200"
    
    # Test public endpoints with real form
    echo
    echo "🌐 PUBLIC FORM TESTS (with real form)"
    echo "===================================="
    
    # Submit to real form (public endpoint) - should work if enabled
    SUBMIT_DATA='{"data":{"name":"John Doe","email":"test@example.com"}}'
    test_http "Submit to Real Form (Public)" "POST" "$SERVER_URL/forms/$CREATED_FORM_ID/submit" "-H 'Content-Type: application/json'" "$SUBMIT_DATA" "201"
    
    # Register event on real form (public endpoint) - should work if enabled
    EVENT_DATA='{"type":"view"}'
    test_http "Register Event on Real Form (Public)" "POST" "$SERVER_URL/forms/$CREATED_FORM_ID/events" "-H 'Content-Type: application/json'" "$EVENT_DATA" "204"
    
    # Delete the real form at the end
    test_http "Delete Real Form" "DELETE" "$SERVER_URL/api/v1/forms/$CREATED_FORM_ID" "$AUTH_HEADER" "" "204"
else
    log_error "Failed to create form or extract ID. Response: $response"
    # Fall back to original tests with fake ID
    test_http "Create Form" "POST" "$SERVER_URL/api/v1/forms" "$AUTH_HEADER -H 'Content-Type: application/json'" "$FORM_DATA" "201"
fi

# List forms
test_http "List Forms" "GET" "$SERVER_URL/api/v1/forms" "$AUTH_HEADER" "" "200"

# Get forms summary (implemented endpoint)
test_http "Get Forms Summary" "GET" "$SERVER_URL/api/v1/forms/summary" "$AUTH_HEADER" "" "200"

# Test with a sample form ID (for endpoints that should return 404)
FAKE_FORM_ID="test-form-123"

# Get non-existent form
test_http "Get Non-existent Form" "GET" "$SERVER_URL/api/v1/forms/$FAKE_FORM_ID" "$AUTH_HEADER" "" "404"

# Update non-existent form
UPDATE_DATA='{"name":"Updated Test Form","type":"contact"}'
test_http "Update Non-existent Form" "PUT" "$SERVER_URL/api/v1/forms/$FAKE_FORM_ID" "$AUTH_HEADER -H 'Content-Type: application/json'" "$UPDATE_DATA" "404"

# Get stats for non-existent form
test_http "Get Non-existent Form Stats" "GET" "$SERVER_URL/api/v1/forms/$FAKE_FORM_ID/stats" "$AUTH_HEADER" "" "404"

# Get submissions for non-existent form
test_http "Get Non-existent Form Submissions" "GET" "$SERVER_URL/api/v1/forms/$FAKE_FORM_ID/submissions" "$AUTH_HEADER" "" "404"

# Delete non-existent form
test_http "Delete Non-existent Form" "DELETE" "$SERVER_URL/api/v1/forms/$FAKE_FORM_ID" "$AUTH_HEADER" "" "404"

# Test 4: Public Form Endpoints (Legacy tests with fake ID)
echo "🌐 PUBLIC FORM TESTS (Legacy)"
echo "============================"

FAKE_ID="nonexistent-form-id"

# Submit to non-existent form (public endpoint)
SUBMIT_DATA='{"data":{"email":"test@example.com","message":"Test submission"}}'
test_http "Submit to Non-existent Form (Public)" "POST" "$SERVER_URL/forms/$FAKE_ID/submit" "-H 'Content-Type: application/json'" "$SUBMIT_DATA" "404"

# Register event on non-existent form (public endpoint)
EVENT_DATA='{"type":"view"}'
test_http "Register Event on Non-existent Form (Public)" "POST" "$SERVER_URL/forms/$FAKE_ID/events" "-H 'Content-Type: application/json'" "$EVENT_DATA" "404"

# Test 5: User Management Endpoints
echo "👤 USER MANAGEMENT TESTS"
echo "======================="

# Test user TTL update - should fail with 403 (forbidden) because user can't update TTL for other users
USER_ID="test-user-123"
TTL_DATA='{"ttl_days":365}'
test_http "Update User TTL (should be forbidden)" "PUT" "$SERVER_URL/api/v1/users/$USER_ID/ttl" "$AUTH_HEADER -H 'Content-Type: application/json'" "$TTL_DATA" "403"

# Test user TTL update for same user - should work
SAME_USER_ID="e2e-test-user"  # Same as token user
test_http "Update Own User TTL" "PUT" "$SERVER_URL/api/v1/users/$SAME_USER_ID/ttl" "$AUTH_HEADER -H 'Content-Type: application/json'" "$TTL_DATA" "200"

# Test 6: Rate Limiting Tests (Public endpoints)
echo "⏱️  RATE LIMITING TESTS"
echo "======================"

log_info "Testing rate limiting on public endpoints..."
log_info "Sending multiple requests to public endpoint..."

# Test rate limiting by sending multiple requests quickly
for i in {1..5}; do
    response=$(curl -s -w '\nHTTP_CODE:%{http_code}\n' -X POST \
        -H 'Content-Type: application/json' \
        -d '{"test":"data"}' \
        "$SERVER_URL/forms/nonexistent/submit")
    
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    
    if [ "$http_code" = "429" ]; then
        log_success "Rate limiting working (HTTP 429 on request $i)"
        break
    elif [ "$i" = "5" ]; then
        log_warning "Rate limiting not triggered after 5 requests (this may be expected)"
    fi
done
echo

# Test 7: Private endpoints should NOT be rate limited
echo "🔓 PRIVATE ENDPOINT RATE LIMIT BYPASS TESTS"
echo "==========================================="

log_info "Testing that private endpoints bypass rate limiting..."
log_info "Sending multiple authenticated requests quickly..."

SUCCESS_COUNT=0
for i in {1..10}; do
    response=$(curl -s -w '\nHTTP_CODE:%{http_code}\n' \
        -H "Authorization: Bearer $TOKEN" \
        "$SERVER_URL/api/v1/forms")
    
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    
    if [ "$http_code" = "200" ]; then
        ((SUCCESS_COUNT++))
    elif [ "$http_code" = "429" ]; then
        log_error "Private endpoint got rate limited (should not happen)"
        break
    fi
done

if [ "$SUCCESS_COUNT" = "10" ]; then
    log_success "All 10 authenticated requests succeeded (no rate limiting)"
    ((PASSED_TESTS++))
else
    log_error "Only $SUCCESS_COUNT/10 authenticated requests succeeded"
    ((FAILED_TESTS++))
fi
((TOTAL_TESTS++))
echo

# Test 8: Invalid HTTP Methods
echo "� INVALID METHOD TESTS"
echo "======================"

test_http "Invalid method on forms endpoint" "PATCH" "$SERVER_URL/api/v1/forms" "$AUTH_HEADER" "" "405"
test_http "Invalid method on health endpoint" "POST" "$SERVER_URL/health" "" "" "405"

# Test 9: Invalid Endpoints
echo "🔍 INVALID ENDPOINT TESTS"
echo "========================"

test_http "Non-existent endpoint" "GET" "$SERVER_URL/nonexistent" "" "" "404"
test_http "Non-existent private endpoint" "GET" "$SERVER_URL/api/v1/invalidpath" "$AUTH_HEADER" "" "404"

# Test 10: Event Statistics Tests
echo "📈 EVENT STATISTICS TESTS"
echo "========================"

# Create a form specifically for statistics testing
STATS_FORM_DATA='{"name":"Statistics Test Form","type":"contact","enabled":true,"fields":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}'
log_info "Creating form for statistics testing..."
stats_response=$(curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$STATS_FORM_DATA" "$SERVER_URL/api/v1/forms")
STATS_FORM_ID=$(echo "$stats_response" | jq -r '.data.id // empty')

if [ -n "$STATS_FORM_ID" ] && [ "$STATS_FORM_ID" != "null" ]; then
    log_success "Statistics test form created with ID: $STATS_FORM_ID"
    
    # DEBUG: Check the actual form data returned
    log_info "Form creation response: $stats_response"
    form_enabled=$(echo "$stats_response" | jq -r '.data.enabled // "undefined"')
    log_info "Form enabled status after creation: $form_enabled"
    
    # Additional check: Get the form directly to verify its state
    log_info "Verifying form state with direct GET..."
    direct_form_response=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID")
    direct_enabled=$(echo "$direct_form_response" | jq -r '.data.enabled // "undefined"')
    log_info "Direct form check enabled status: $direct_enabled"
    log_info "Direct form response: $direct_form_response"
    
    # If form is not enabled, try to enable it
    if [ "$direct_enabled" != "true" ]; then
        log_warning "Form is not enabled ($direct_enabled), attempting to enable it..."
        enable_response=$(curl -s -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"enabled":true}' "$SERVER_URL/api/v1/forms/$STATS_FORM_ID")
        log_info "Enable response: $enable_response"
        
        # Check again
        recheck_response=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID")
        recheck_enabled=$(echo "$recheck_response" | jq -r '.data.enabled // "undefined"')
        log_info "After enable attempt, form enabled status: $recheck_enabled"
        
        if [ "$recheck_enabled" != "true" ]; then
            log_error "Unable to enable form for statistics testing. Form state: $recheck_enabled"
            log_error "This will prevent public endpoint event registration."
        else
            log_success "Successfully enabled form for statistics testing"
        fi
    else
        log_success "Form is already enabled for statistics testing"
    fi
    
    # First, get initial stats (should be empty/zero)
    log_info "Checking initial statistics..."
    initial_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID/stats")
    echo "Initial stats: $initial_stats"
    
    # Test 1: Register view events and check statistics
    log_info "Testing view events statistics..."
    for i in {1..3}; do
        view_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST -H 'Content-Type: application/json' -d '{"type":"view"}' "$SERVER_URL/forms/$STATS_FORM_ID/events")
        http_status=$(echo "$view_response" | grep "HTTP_STATUS:" | cut -d: -f2)
        response_body=$(echo "$view_response" | sed '/HTTP_STATUS:/d')
        log_info "View event $i: HTTP $http_status"
        if [ "$http_status" != "204" ]; then
            log_warning "View event $i failed with status $http_status: $response_body"
        fi
        sleep 0.1
    done
    
    # Check stats after view events
    view_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID/stats")
    view_count=$(echo "$view_stats" | jq -r '.data.views // 0')
    
    if [ "$view_count" = "3" ]; then
        log_success "View events correctly counted: $view_count views"
    else
        log_error "View events not counted correctly. Expected: 3, Got: $view_count"
        echo "Stats response: $view_stats"
    fi
    
    # Test 2: Register submission and check statistics
    log_info "Testing submission statistics..."
    submit_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST -H 'Content-Type: application/json' -d '{"data":{"name":"John Doe","email":"john@test.com"}}' "$SERVER_URL/forms/$STATS_FORM_ID/submit")
    submit_http_status=$(echo "$submit_response" | grep "HTTP_STATUS:" | cut -d: -f2)
    submit_response_body=$(echo "$submit_response" | sed '/HTTP_STATUS:/d')
    log_info "Submit response HTTP status: $submit_http_status"
    if [ "$submit_http_status" != "201" ]; then
        log_warning "Submission failed with status $submit_http_status: $submit_response_body"
    else
        log_info "Submission successful: $submit_response_body"
    fi
    
    # Check stats after submission
    submit_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID/stats")
    submit_count=$(echo "$submit_stats" | jq -r '.data.submits // 0')
    
    if [ "$submit_count" = "1" ]; then
        log_success "Submission correctly counted: $submit_count submission"
    else
        log_error "Submission not counted correctly. Expected: 1, Got: $submit_count"
        echo "Stats response: $submit_stats"
    fi
    
    # Test 3: Register close events and check statistics
    log_info "Testing close events statistics..."
    for i in {1..2}; do
        close_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST -H 'Content-Type: application/json' -d '{"type":"close"}' "$SERVER_URL/forms/$STATS_FORM_ID/events")
        close_http_status=$(echo "$close_response" | grep "HTTP_STATUS:" | cut -d: -f2)
        close_response_body=$(echo "$close_response" | sed '/HTTP_STATUS:/d')
        log_info "Close event $i: HTTP $close_http_status"
        if [ "$close_http_status" != "204" ]; then
            log_warning "Close event $i failed with status $close_http_status: $close_response_body"
        fi
        sleep 0.1
    done
    
    # Check final comprehensive stats
    final_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID/stats")
    final_views=$(echo "$final_stats" | jq -r '.data.views // 0')
    final_submits=$(echo "$final_stats" | jq -r '.data.submits // 0')
    final_closes=$(echo "$final_stats" | jq -r '.data.closes // 0')
    
    log_info "Final comprehensive statistics check..."
    echo "📊 Final Statistics Summary:"
    echo "  Views: $final_views (expected: 3)"
    echo "  Submits: $final_submits (expected: 1)"
    echo "  Closes: $final_closes (expected: 2)"
    echo "Full stats response: $final_stats"
    
    # Validate all statistics
    stats_correct=true
    if [ "$final_views" != "3" ]; then
        log_error "Views count incorrect. Expected: 3, Got: $final_views"
        stats_correct=false
    fi
    if [ "$final_submits" != "1" ]; then
        log_error "Submits count incorrect. Expected: 1, Got: $final_submits"
        stats_correct=false
    fi
    if [ "$final_closes" != "2" ]; then
        log_error "Closes count incorrect. Expected: 2, Got: $final_closes"
        stats_correct=false
    fi
    
    if [ "$stats_correct" = true ]; then
        log_success "All event statistics are correctly tracked!"
        ((PASSED_TESTS++))
    else
        log_error "Some statistics are incorrect"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
    
    # Test 4: Check if statistics persist across requests
    log_info "Testing statistics persistence..."
    persistence_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID/stats")
    persistence_views=$(echo "$persistence_stats" | jq -r '.data.views // 0')
    
    if [ "$persistence_views" = "3" ]; then
        log_success "Statistics persist correctly across requests"
        ((PASSED_TESTS++))
    else
        log_error "Statistics persistence failed. Expected: 3, Got: $persistence_views"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
    
    # Clean up: delete the statistics test form
    delete_response=$(curl -s -X DELETE -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/forms/$STATS_FORM_ID")
    log_info "Cleaned up statistics test form"
    
else
    log_error "Failed to create statistics test form. Skipping statistics tests."
    echo "Form creation response: $stats_response"
    ((FAILED_TESTS+=4))
    ((TOTAL_TESTS+=4))
fi

echo
echo "📄 CONTENT TYPE TESTS"
echo "===================="

test_http "JSON endpoint with wrong content type" "POST" "$SERVER_URL/api/v1/forms" "$AUTH_HEADER -H 'Content-Type: text/plain'" "invalid data" "400"

# Final Results
echo "🏁 TEST SUMMARY"
echo "==============="
echo -e "${BLUE}Total Tests: $TOTAL_TESTS${NC}"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"

if [ "$FAILED_TESTS" -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}💥 Some tests failed!${NC}"
    exit 1
fi
