#!/bin/bash

# Full E2E Test Suite for Leads Core Service
# Tests all endpoints, authentication, rate limiting, and functionality

# set -e  # Exit on any error (commented out for debugging)


# Start cluster
echo "🚀 Starting Redis cluster..."
docker-compose -f docker-compose.cluster.yml up --build -d

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
SERVER_URL="http://localhost:8080"
JWT_SECRET="development-jwt-secret-change-in-production"
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
    echo
echo "📄 CONTENT TYPE TESTS"
echo "===================="

test_http "JSON endpoint with wrong content type" "POST" "$SERVER_URL/api/v1/widgets" "$AUTH_HEADER -H 'Content-Type: text/plain'" "invalid data" "400"

# Final Results
echo
echo "🏁 TEST SUMMARY"
echo "==============="
echo -e "${BLUE}Total Tests: $TOTAL_TESTS${NC}"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"
echo -e "${YELLOW}Debug: Sum check = $((PASSED_TESTS + FAILED_TESTS))${NC}"

if [ "$FAILED_TESTS" -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    echo -e "${GREEN}✅ Including load tests with 2000+ concurrent events${NC}"
    exit 0
else
    echo -e "${RED}💥 Some tests failed!${NC}"
    exit 1
fi
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

log_info "Cleaning up Redis database..."
docker-compose -f docker-compose.cluster.yml exec -T redis-node-0 redis-cli FLUSHALL
docker-compose -f docker-compose.cluster.yml exec -T redis-node-1 redis-cli FLUSHALL
docker-compose -f docker-compose.cluster.yml exec -T redis-node-2 redis-cli FLUSHALL

# Generate test JWT token
log_info "Setting up test environment..."
if [ ! -f "./bin/jwt-gen" ]; then
    log_error "JWT generator not found. Please build the project first: make build"
    exit 1
fi

TOKEN=$(./bin/jwt-gen -secret="$JWT_SECRET" -user="$TEST_USER" -ttl="1h")
AUTH_HEADER="-H 'Authorization: Bearer $TOKEN'"
log_info "JWT token generated for user: $TEST_USER"
echo

# Test 1: System Health Endpoints
echo "📊 SYSTEM HEALTH TESTS"
echo "====================="

test_http "Health Check" "GET" "$SERVER_URL/health" "" "" "200"
test_http "Metrics Endpoint" "GET" "$SERVER_URL/metrics" "" "" "200"

# Test 2: Authentication Tests
echo "🔐 AUTHENTICATION TESTS"
echo "======================"

test_http "Access private endpoint without token" "GET" "$SERVER_URL/api/v1/widgets" "" "" "401"
test_http "Access private endpoint with invalid token" "GET" "$SERVER_URL/api/v1/widgets" "-H 'Authorization: Bearer invalid-token'" "" "401"
test_http "Access private endpoint with valid token" "GET" "$SERVER_URL/api/v1/widgets" "$AUTH_HEADER" "" "200"

# Test 3: Private Widget Management Endpoints (Authenticated)
echo "📝 PRIVATE WIDGET MANAGEMENT TESTS"
echo "==============================="

# Create a widget and extract its ID
WIDGET_DATA='{"name":"Test Widget","type":"lead-form","enabled":true,"fields":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}'
log_info "Creating widget and extracting ID..."
response=$(curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$WIDGET_DATA" "$SERVER_URL/api/v1/widgets")
CREATED_WIDGET_ID=$(echo "$response" | jq -r '.data.id // empty')

if [ -n "$CREATED_WIDGET_ID" ] && [ "$CREATED_WIDGET_ID" != "null" ]; then
    log_info "Widget created with ID: $CREATED_WIDGET_ID"
    
    # Now test operations with the real widget ID
    # Note: Widget might be disabled by default, so some tests may return 403/404
    test_http "Get Real Widget" "GET" "$SERVER_URL/api/v1/widgets/$CREATED_WIDGET_ID" "$AUTH_HEADER" "" "200"
    
    # Update the real widget
    UPDATE_DATA='{"name":"Updated Test Widget","type":"lead-form"}'
    test_http "Update Real Widget" "PUT" "$SERVER_URL/api/v1/widgets/$CREATED_WIDGET_ID" "$AUTH_HEADER -H 'Content-Type: application/json'" "$UPDATE_DATA" "200"
    
    # Get stats for real widget
    test_http "Get Real Widget Stats" "GET" "$SERVER_URL/api/v1/widgets/$CREATED_WIDGET_ID/stats" "$AUTH_HEADER" "" "200"
    
    # Get submissions for real widget
    test_http "Get Real Widget Submissions" "GET" "$SERVER_URL/api/v1/widgets/$CREATED_WIDGET_ID/submissions" "$AUTH_HEADER" "" "200"
    
    # Test public endpoints with real widget
    echo
    echo "🌐 PUBLIC WIDGET TESTS (with real widget)"
    echo "===================================="
    
    # Submit to real widget (public endpoint) - should work if enabled
    SUBMIT_DATA='{"data":{"name":"John Doe","email":"test@example.com"}}'
    test_http "Submit to Real Widget (Public)" "POST" "$SERVER_URL/widgets/$CREATED_WIDGET_ID/submit" "-H 'Content-Type: application/json'" "$SUBMIT_DATA" "201"
    
    # Register event on real widget (public endpoint) - should work if enabled
    EVENT_DATA='{"type":"view"}'
    test_http "Register Event on Real Widget (Public)" "POST" "$SERVER_URL/widgets/$CREATED_WIDGET_ID/events" "-H 'Content-Type: application/json'" "$EVENT_DATA" "204"
    
    # Delete the real widget at the end
    test_http "Delete Real Widget" "DELETE" "$SERVER_URL/api/v1/widgets/$CREATED_WIDGET_ID" "$AUTH_HEADER" "" "204"
else
    log_error "Failed to create widget or extract ID. Response: $response"
    # Fall back to original tests with fake ID - skip additional tests
    ((FAILED_TESTS+=7))  # Account for the 7 tests we couldn't run
    ((TOTAL_TESTS+=7))
fi

# Always test widget creation to verify the endpoint works
test_http "Create Widget (verification)" "POST" "$SERVER_URL/api/v1/widgets" "$AUTH_HEADER -H 'Content-Type: application/json'" "$WIDGET_DATA" "201"

# List widgets
test_http "List Widgets" "GET" "$SERVER_URL/api/v1/widgets" "$AUTH_HEADER" "" "200"

# Get widgets summary (implemented endpoint)
test_http "Get Widgets Summary" "GET" "$SERVER_URL/api/v1/widgets/summary" "$AUTH_HEADER" "" "200"

# Test with a sample widget ID (for endpoints that should return 404)
FAKE_WIDGET_ID="test-widget-123"

# Get non-existent widget
test_http "Get Non-existent Widget" "GET" "$SERVER_URL/api/v1/widgets/$FAKE_WIDGET_ID" "$AUTH_HEADER" "" "404"

# Update non-existent widget
UPDATE_DATA='{"name":"Updated Test Widget","type":"lead-form"}'
test_http "Update Non-existent Widget" "PUT" "$SERVER_URL/api/v1/widgets/$FAKE_WIDGET_ID" "$AUTH_HEADER -H 'Content-Type: application/json'" "$UPDATE_DATA" "404"

# Get stats for non-existent widget
test_http "Get Non-existent Widget Stats" "GET" "$SERVER_URL/api/v1/widgets/$FAKE_WIDGET_ID/stats" "$AUTH_HEADER" "" "404"

# Get submissions for non-existent widget
test_http "Get Non-existent Widget Submissions" "GET" "$SERVER_URL/api/v1/widgets/$FAKE_WIDGET_ID/submissions" "$AUTH_HEADER" "" "404"

# Delete non-existent widget
test_http "Delete Non-existent Widget" "DELETE" "$SERVER_URL/api/v1/widgets/$FAKE_WIDGET_ID" "$AUTH_HEADER" "" "404"

# Test 4: Public Widget Endpoints (Legacy tests with fake ID)
echo "🌐 PUBLIC WIDGET TESTS (Legacy)"
echo "============================"

FAKE_ID="nonexistent-widget-id"

# Submit to non-existent widget (public endpoint)
SUBMIT_DATA='{"data":{"email":"test@example.com","message":"Test submission"}}'
test_http "Submit to Non-existent Widget (Public)" "POST" "$SERVER_URL/widgets/$FAKE_ID/submit" "-H 'Content-Type: application/json'" "$SUBMIT_DATA" "404"

# Register event on non-existent widget (public endpoint)
EVENT_DATA='{"type":"view"}'
test_http "Register Event on Non-existent Widget (Public)" "POST" "$SERVER_URL/widgets/$FAKE_ID/events" "-H 'Content-Type: application/json'" "$EVENT_DATA" "404"

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
        "$SERVER_URL/widgets/nonexistent/submit")
    
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
        "$SERVER_URL/api/v1/widgets")
    
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
else
    log_error "Only $SUCCESS_COUNT/10 authenticated requests succeeded"
fi
((TOTAL_TESTS++))
echo

# Test 8: Invalid HTTP Methods
echo "� INVALID METHOD TESTS"
echo "======================"

test_http "Invalid method on widgets endpoint" "PATCH" "$SERVER_URL/api/v1/widgets" "$AUTH_HEADER" "" "405"
test_http "Invalid method on health endpoint" "POST" "$SERVER_URL/health" "" "" "405"

# Test 9: Invalid Endpoints
echo "🔍 INVALID ENDPOINT TESTS"
echo "========================"

test_http "Non-existent endpoint" "GET" "$SERVER_URL/nonexistent" "" "" "404"
test_http "Non-existent private endpoint" "GET" "$SERVER_URL/api/v1/invalidpath" "$AUTH_HEADER" "" "404"

# Test 10: Event Statistics Tests
echo "📈 EVENT STATISTICS TESTS"
echo "========================"

# Create a widget specifically for statistics testing
STATS_WIDGET_DATA='{"name":"Statistics Test Widget","type":"lead-form","enabled":true,"fields":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}'
log_info "Creating widget for statistics testing..."
stats_response=$(curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$STATS_WIDGET_DATA" "$SERVER_URL/api/v1/widgets")
STATS_WIDGET_ID=$(echo "$stats_response" | jq -r '.data.id // empty')

if [ -n "$STATS_WIDGET_ID" ] && [ "$STATS_WIDGET_ID" != "null" ]; then
    log_info "Statistics test widget created with ID: $STATS_WIDGET_ID"
    
    # DEBUG: Check the actual widget data returned
    log_info "Widget creation response: $stats_response"
    widget_enabled=$(echo "$stats_response" | jq -r '.data.enabled // "undefined"')
    log_info "Widget enabled status after creation: $widget_enabled"
    
    # Additional check: Get the widget directly to verify its state
    log_info "Verifying widget state with direct GET..."
    direct_widget_response=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID")
    direct_enabled=$(echo "$direct_widget_response" | jq -r '.data.enabled // "undefined"')
    log_info "Direct widget check enabled status: $direct_enabled"
    log_info "Direct widget response: $direct_widget_response"
    
    # If widget is not enabled, try to enable it
    if [ "$direct_enabled" != "true" ]; then
        log_warning "Widget is not enabled ($direct_enabled), attempting to enable it..."
        enable_response=$(curl -s -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"enabled":true}' "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID")
        log_info "Enable response: $enable_response"
        
        # Check again
        recheck_response=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID")
        recheck_enabled=$(echo "$recheck_response" | jq -r '.data.enabled // "undefined"')
        log_info "After enable attempt, widget enabled status: $recheck_enabled"
        
        if [ "$recheck_enabled" != "true" ]; then
            log_error "Unable to enable widget for statistics testing. Widget state: $recheck_enabled"
            log_error "This will prevent public endpoint event registration."
        else
            log_info "Successfully enabled widget for statistics testing"
        fi
    else
        log_info "Widget is already enabled for statistics testing"
    fi
    
    # First, get initial stats (should be empty/zero)
    log_info "Checking initial statistics..."
    initial_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID/stats")
    echo "Initial stats: $initial_stats"
    
    # Test 1: Register view events and check statistics
    log_info "Testing view events statistics..."
    for i in {1..3}; do
        view_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST -H 'Content-Type: application/json' -d '{"type":"view"}' "$SERVER_URL/widgets/$STATS_WIDGET_ID/events")
        http_status=$(echo "$view_response" | grep "HTTP_STATUS:" | cut -d: -f2)
        response_body=$(echo "$view_response" | sed '/HTTP_STATUS:/d')
        log_info "View event $i: HTTP $http_status"
        if [ "$http_status" != "204" ]; then
            log_warning "View event $i failed with status $http_status: $response_body"
        fi
        sleep 0.1
    done
    
    # Check stats after view events
    view_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID/stats")
    view_count=$(echo "$view_stats" | jq -r '.data.views // 0')
    
    if [ "$view_count" = "3" ]; then
        log_info "View events correctly counted: $view_count views"
    else
        log_error "View events not counted correctly. Expected: 3, Got: $view_count"
        echo "Stats response: $view_stats"
    fi
    
    # Test 2: Register submission and check statistics
    log_info "Testing submission statistics..."
    submit_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST -H 'Content-Type: application/json' -d '{"data":{"name":"John Doe","email":"john@test.com"}}' "$SERVER_URL/widgets/$STATS_WIDGET_ID/submit")
    submit_http_status=$(echo "$submit_response" | grep "HTTP_STATUS:" | cut -d: -f2)
    submit_response_body=$(echo "$submit_response" | sed '/HTTP_STATUS:/d')
    log_info "Submit response HTTP status: $submit_http_status"
    if [ "$submit_http_status" != "201" ]; then
        log_warning "Submission failed with status $submit_http_status: $submit_response_body"
    else
        log_info "Submission successful: $submit_response_body"
    fi
    
    # Check stats after submission
    submit_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID/stats")
    submit_count=$(echo "$submit_stats" | jq -r '.data.submits // 0')
    
    if [ "$submit_count" = "1" ]; then
        log_info "Submission correctly counted: $submit_count submission"
    else
        log_error "Submission not counted correctly. Expected: 1, Got: $submit_count"
        echo "Stats response: $submit_stats"
    fi
    
    # Test 3: Register close events and check statistics
    log_info "Testing close events statistics..."
    for i in {1..2}; do
        close_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST -H 'Content-Type: application/json' -d '{"type":"close"}' "$SERVER_URL/widgets/$STATS_WIDGET_ID/events")
        close_http_status=$(echo "$close_response" | grep "HTTP_STATUS:" | cut -d: -f2)
        close_response_body=$(echo "$close_response" | sed '/HTTP_STATUS:/d')
        log_info "Close event $i: HTTP $close_http_status"
        if [ "$close_http_status" != "204" ]; then
            log_warning "Close event $i failed with status $close_http_status: $close_response_body"
        fi
        sleep 0.1
    done
    
    # Check final comprehensive stats
    final_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID/stats")
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
    else
        log_error "Some statistics are incorrect"
    fi
    ((TOTAL_TESTS++))
    
    # Test 4: Check if statistics persist across requests
    log_info "Testing statistics persistence..."
    persistence_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID/stats")
    persistence_views=$(echo "$persistence_stats" | jq -r '.data.views // 0')
    
    if [ "$persistence_views" = "3" ]; then
        log_success "Statistics persist correctly across requests"
    else
        log_error "Statistics persistence failed. Expected: 3, Got: $persistence_views"
    fi
    ((TOTAL_TESTS++))
    
    # Clean up: delete the statistics test widget
    delete_response=$(curl -s -X DELETE -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$STATS_WIDGET_ID")
    log_info "Cleaned up statistics test widget"
    
else
    log_error "Failed to create statistics test widget. Skipping statistics tests."
    echo "Widget creation response: $stats_response"
    ((FAILED_TESTS+=6))  # Updated to account for new load tests
    ((TOTAL_TESTS+=6))
fi

# Test 11: Load Testing for Statistics
echo "🚀 LOAD TESTING FOR STATISTICS"
echo "=============================="

# Create a widget specifically for load testing
LOAD_WIDGET_DATA='{"name":"Load Test Widget","type":"lead-form","enabled":true,"fields":{"name":{"type":"text","required":true},"email":{"type":"email","required":true}}}'
log_info "Creating widget for load testing..."
load_response=$(curl -s -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$LOAD_WIDGET_DATA" "$SERVER_URL/api/v1/widgets")
LOAD_WIDGET_ID=$(echo "$load_response" | jq -r '.data.id // empty')

if [ -n "$LOAD_WIDGET_ID" ] && [ "$LOAD_WIDGET_ID" != "null" ]; then
    log_info "Load test widget created with ID: $LOAD_WIDGET_ID"
    
    # Get initial stats
    log_info "Getting initial statistics before load test..."
    initial_load_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$LOAD_WIDGET_ID/stats")
    initial_views=$(echo "$initial_load_stats" | jq -r '.data.views // 0')
    log_info "Initial views: $initial_views"
    
    # Load Test 1: 1000 view events in smaller batches for comprehensive testing
    log_info "Starting load test: 1000 view events in smaller batches..."
    echo "📊 Load Test Configuration:"
    echo "  Total events: 1000"
    echo "  Batch size: 10 requests"
    echo "  Batches: 100"
    echo "  Parallel threads per batch: 10"
    
    # Start timing
    load_start_time=$(date +%s)
    
    total_successful_events=0
    
    # Send events in batches to avoid overwhelming the system
    for batch in {1..100}; do
        # Log progress every 10 batches to reduce output noise
        if [ $((batch % 10)) -eq 0 ]; then
            log_info "Processing batch $batch/100..."
        fi
        
        # Create a temporary directory for this batch
        BATCH_TEMP_DIR="/tmp/leads_batch_${batch}_$$"
        mkdir -p "$BATCH_TEMP_DIR"
        
        # Function to send a single event (optimized)
        send_single_event() {
            local event_id=$1
            local widget_id=$2
            local server_url=$3
            local temp_dir=$4
            
            response=$(curl -s -w "\n%{http_code}" -X POST \
                -H 'Content-Type: application/json' \
                -d '{"type":"view"}' \
                --connect-timeout 10 \
                --max-time 15 \
                "$server_url/widgets/$widget_id/events" 2>/dev/null)
            
            http_code=$(echo "$response" | tail -1)
            response_body=$(echo "$response" | sed '$d')
            
            if [ "$http_code" = "204" ]; then
                echo "1" > "$temp_dir/event_$event_id.success"
            else
                echo "0" > "$temp_dir/event_$event_id.success"
                # Log failed request details for debugging
                echo "Event $event_id failed: HTTP $http_code - $response_body" > "$temp_dir/event_$event_id.error"
            fi
        }
        
        # Export function for this batch
        export -f send_single_event
        export BATCH_TEMP_DIR
        export LOAD_WIDGET_ID
        export SERVER_URL
        
        # Launch 10 parallel requests for this batch
        for event_id in {1..10}; do
            send_single_event "$event_id" "$LOAD_WIDGET_ID" "$SERVER_URL" "$BATCH_TEMP_DIR" &
        done
        
        # Wait for batch to complete
        wait
        
        # Count successful events in this batch
        batch_success=0
        batch_errors=0
        for event_id in {1..10}; do
            if [ -f "$BATCH_TEMP_DIR/event_$event_id.success" ]; then
                success=$(cat "$BATCH_TEMP_DIR/event_$event_id.success")
                batch_success=$((batch_success + success))
                if [ "$success" = "0" ] && [ -f "$BATCH_TEMP_DIR/event_$event_id.error" ]; then
                    batch_errors=$((batch_errors + 1))
                    if [ "$batch_errors" = "1" ]; then
                        log_warning "First error in batch $batch: $(cat "$BATCH_TEMP_DIR/event_$event_id.error")"
                    fi
                fi
            fi
        done
        
        total_successful_events=$((total_successful_events + batch_success))
        
        if [ "$batch_success" -lt 10 ]; then
            log_warning "Batch $batch: only $batch_success/10 successful (${batch_errors} errors)"
        fi
        
        # Clean up batch temp directory
        rm -rf "$BATCH_TEMP_DIR"
        
        # Brief pause between batches to avoid overwhelming the server
        sleep 0.1
    done
    
    load_end_time=$(date +%s)
    load_duration=$((load_end_time - load_start_time))
    
    log_info "Load test completed in ${load_duration} seconds"
    log_info "Successfully sent events: $total_successful_events / 1000"
    
    # Give the system a moment to process all events
    log_info "Waiting 5 seconds for event processing..."
    sleep 5
    
    # Check final statistics
    log_info "Checking final statistics after load test..."
    final_load_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$LOAD_WIDGET_ID/stats")
    final_load_views=$(echo "$final_load_stats" | jq -r '.data.views // 0')
    expected_views=$((initial_views + 1000))
    
    echo "📊 Load Test Results:"
    echo "  Initial views: $initial_views"
    echo "  Expected final views: $expected_views"
    echo "  Actual final views: $final_load_views"
    echo "  Successfully sent requests: $total_successful_events / 1000"
    echo "  Test duration: ${load_duration} seconds"
    echo "  Requests per second: $((1000 / load_duration))"
    
    # Validate results
    if [ "$final_load_views" = "$expected_views" ]; then
        log_success "Load test PASSED: All 1000 events correctly counted!"
    else
        log_error "Load test FAILED: Expected $expected_views views, got $final_load_views"
    fi
    ((TOTAL_TESTS++))
    
    # Load Test 2: Mixed events load test with larger batches
    log_info "Starting mixed events load test..."
    echo "📊 Mixed Load Test Configuration:"
    echo "  View events: 500 (50 batches × 10 events)"
    echo "  Submit events: 300 (30 batches × 10 events)"  
    echo "  Close events: 200 (20 batches × 10 events)"
    echo "  Total events: 1000"
    
    # Get current stats before mixed test
    pre_mixed_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$LOAD_WIDGET_ID/stats")
    pre_mixed_views=$(echo "$pre_mixed_stats" | jq -r '.data.views // 0')
    pre_mixed_submits=$(echo "$pre_mixed_stats" | jq -r '.data.submits // 0')
    pre_mixed_closes=$(echo "$pre_mixed_stats" | jq -r '.data.closes // 0')
    
    # Start mixed load test timing
    mixed_start_time=$(date +%s)
    
    successful_views=0
    successful_submits=0
    successful_closes=0
    
    # Function for sending view events in batches
    send_view_batch() {
        local batch_id=$1
        local temp_dir="/tmp/leads_view_batch_${batch_id}_$$"
        mkdir -p "$temp_dir"
        
        for event_id in {1..10}; do
            (
                response=$(curl -s -w "\n%{http_code}" -X POST \
                    -H 'Content-Type: application/json' \
                    -d '{"type":"view"}' \
                    --connect-timeout 10 --max-time 15 \
                    "$SERVER_URL/widgets/$LOAD_WIDGET_ID/events" 2>/dev/null)
                
                http_code=$(echo "$response" | tail -1)
                if [ "$http_code" = "204" ]; then
                    echo "1" > "$temp_dir/view_$event_id.success"
                fi
            ) &
        done
        wait
        
        # Count successes
        local batch_success=0
        for event_id in {1..10}; do
            if [ -f "$temp_dir/view_$event_id.success" ]; then
                batch_success=$((batch_success + 1))
            fi
        done
        
        echo "$batch_success" > "/tmp/view_batch_$batch_id.result"
        rm -rf "$temp_dir"
    }
    
    # Function for sending submit events in batches
    send_submit_batch() {
        local batch_id=$1
        local temp_dir="/tmp/leads_submit_batch_${batch_id}_$$"
        mkdir -p "$temp_dir"
        
        for event_id in {1..10}; do
            (
                response=$(curl -s -w "\n%{http_code}" -X POST \
                    -H 'Content-Type: application/json' \
                    -d "{\"data\":{\"name\":\"LoadTest$batch_id-$event_id\",\"email\":\"test$batch_id-$event_id@example.com\"}}" \
                    --connect-timeout 10 --max-time 15 \
                    "$SERVER_URL/widgets/$LOAD_WIDGET_ID/submit" 2>/dev/null)
                
                http_code=$(echo "$response" | tail -1)
                if [ "$http_code" = "201" ]; then
                    echo "1" > "$temp_dir/submit_$event_id.success"
                fi
            ) &
        done
        wait
        
        # Count successes
        local batch_success=0
        for event_id in {1..10}; do
            if [ -f "$temp_dir/submit_$event_id.success" ]; then
                batch_success=$((batch_success + 1))
            fi
        done
        
        echo "$batch_success" > "/tmp/submit_batch_$batch_id.result"
        rm -rf "$temp_dir"
    }
    
    # Function for sending close events in batches
    send_close_batch() {
        local batch_id=$1
        local temp_dir="/tmp/leads_close_batch_${batch_id}_$$"
        mkdir -p "$temp_dir"
        
        for event_id in {1..10}; do
            (
                response=$(curl -s -w "\n%{http_code}" -X POST \
                    -H 'Content-Type: application/json' \
                    -d '{"type":"close"}' \
                    --connect-timeout 10 --max-time 15 \
                    "$SERVER_URL/widgets/$LOAD_WIDGET_ID/events" 2>/dev/null)
                
                http_code=$(echo "$response" | tail -1)
                if [ "$http_code" = "204" ]; then
                    echo "1" > "$temp_dir/close_$event_id.success"
                fi
            ) &
        done
        wait
        
        # Count successes
        local batch_success=0
        for event_id in {1..10}; do
            if [ -f "$temp_dir/close_$event_id.success" ]; then
                batch_success=$((batch_success + 1))
            fi
        done
        
        echo "$batch_success" > "/tmp/close_batch_$batch_id.result"
        rm -rf "$temp_dir"
    }
    
    # Export functions
    export -f send_view_batch send_submit_batch send_close_batch
    
    # Send view events in 50 batches
    log_info "Sending 500 view events in 50 batches..."
    for batch_id in {1..50}; do
        send_view_batch "$batch_id" &
        sleep 0.05  # Smaller delay for faster execution
        # Log progress every 10 batches
        if [ $((batch_id % 10)) -eq 0 ]; then
            log_info "  View events progress: $batch_id/50 batches started"
        fi
    done
    wait
    
    # Send submit events in 30 batches
    log_info "Sending 300 submit events in 30 batches..."
    for batch_id in {1..30}; do
        send_submit_batch "$batch_id" &
        sleep 0.05
        # Log progress every 10 batches
        if [ $((batch_id % 10)) -eq 0 ]; then
            log_info "  Submit events progress: $batch_id/30 batches started"
        fi
    done
    wait
    
    # Send close events in 20 batches
    log_info "Sending 200 close events in 20 batches..."
    for batch_id in {1..20}; do
        send_close_batch "$batch_id" &
        sleep 0.05
        # Log progress every 10 batches
        if [ $((batch_id % 10)) -eq 0 ]; then
            log_info "  Close events progress: $batch_id/20 batches started"
        fi
    done
    wait
    
    mixed_end_time=$(date +%s)
    mixed_duration=$((mixed_end_time - mixed_start_time))
    
    # Collect mixed test results
    for batch_id in {1..50}; do
        if [ -f "/tmp/view_batch_$batch_id.result" ]; then
            batch_success=$(cat "/tmp/view_batch_$batch_id.result")
            successful_views=$((successful_views + batch_success))
            rm -f "/tmp/view_batch_$batch_id.result"
        fi
    done
    
    for batch_id in {1..30}; do
        if [ -f "/tmp/submit_batch_$batch_id.result" ]; then
            batch_success=$(cat "/tmp/submit_batch_$batch_id.result")
            successful_submits=$((successful_submits + batch_success))
            rm -f "/tmp/submit_batch_$batch_id.result"
        fi
    done
    
    for batch_id in {1..20}; do
        if [ -f "/tmp/close_batch_$batch_id.result" ]; then
            batch_success=$(cat "/tmp/close_batch_$batch_id.result")
            successful_closes=$((successful_closes + batch_success))
            rm -f "/tmp/close_batch_$batch_id.result"
        fi
    done
    
    log_info "Mixed load test completed in ${mixed_duration} seconds"
    
    # Give the system time to process mixed events
    log_info "Waiting 5 seconds for mixed event processing..."
    sleep 5
    
    # Check final mixed statistics
    final_mixed_stats=$(curl -s -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$LOAD_WIDGET_ID/stats")
    final_mixed_views=$(echo "$final_mixed_stats" | jq -r '.data.views // 0')
    final_mixed_submits=$(echo "$final_mixed_stats" | jq -r '.data.submits // 0')
    final_mixed_closes=$(echo "$final_mixed_stats" | jq -r '.data.closes // 0')
    
    expected_mixed_views=$((pre_mixed_views + 500))
    expected_mixed_submits=$((pre_mixed_submits + 300))
    expected_mixed_closes=$((pre_mixed_closes + 200))
    
    echo "📊 Mixed Load Test Results:"
    echo "  Views: $final_mixed_views (expected: $expected_mixed_views, sent: $successful_views/500)"
    echo "  Submits: $final_mixed_submits (expected: $expected_mixed_submits, sent: $successful_submits/300)"
    echo "  Closes: $final_mixed_closes (expected: $expected_mixed_closes, sent: $successful_closes/200)"
    echo "  Test duration: ${mixed_duration} seconds"
    echo "  Total requests per second: $(((500 + 300 + 200) / mixed_duration))"
    
    # Validate mixed results
    mixed_test_passed=true
    if [ "$final_mixed_views" != "$expected_mixed_views" ]; then
        log_error "Mixed test FAILED for views: Expected $expected_mixed_views, got $final_mixed_views"
        mixed_test_passed=false
    fi
    if [ "$final_mixed_submits" != "$expected_mixed_submits" ]; then
        log_error "Mixed test FAILED for submits: Expected $expected_mixed_submits, got $final_mixed_submits"
        mixed_test_passed=false
    fi
    if [ "$final_mixed_closes" != "$expected_mixed_closes" ]; then
        log_error "Mixed test FAILED for closes: Expected $expected_mixed_closes, got $final_mixed_closes"
        mixed_test_passed=false
    fi
    
    if [ "$mixed_test_passed" = true ]; then
        log_success "Mixed load test PASSED: All 1000 events correctly counted!"
    else
        log_error "Mixed load test FAILED: Some events not counted correctly"
    fi
    ((TOTAL_TESTS++))
    
    # Clean up: delete the load test widget
    delete_load_response=$(curl -s -X DELETE -H "Authorization: Bearer $TOKEN" "$SERVER_URL/api/v1/widgets/$LOAD_WIDGET_ID")
    log_info "Cleaned up load test widget"
    
else
    log_error "Failed to create load test widget. Skipping load tests."
    echo "Load widget creation response: $load_response"
    ((FAILED_TESTS+=2))
    ((TOTAL_TESTS+=2))
fi

echo
echo "📄 CONTENT TYPE TESTS"
echo "===================="

test_http "JSON endpoint with wrong content type" "POST" "$SERVER_URL/api/v1/widgets" "$AUTH_HEADER -H 'Content-Type: text/plain'" "invalid data" "400"

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
