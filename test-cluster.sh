#!/bin/bash

# Redis Cluster Test Script
# This script tests Redis cluster functionality with the leads-core application

set -e

JWT_SECRET="development-jwt-secret-change-in-production"
JWT_USER="test-user"

CLUSTER_COMPOSE_FILE="docker-compose.cluster.yml"
APP_URL="http://localhost:8080"

echo "🧪 Redis Cluster Integration Test"
echo "=================================="

# Function to wait for service to be ready
wait_for_service() {
    local url=$1
    local timeout=${2:-60}
    local count=0
    
    echo "⏳ Waiting for service at $url to be ready..."
    
    while [ $count -lt $timeout ]; do
        if curl -s -f "$url/health" > /dev/null 2>&1; then
            echo "✅ Service is ready!"
            return 0
        fi
        count=$((count + 1))
        sleep 1
        echo -n "."
    done
    
    echo "❌ Service failed to start within ${timeout}s"
    return 1
}

# Function to test API endpoint
test_api() {
    local method=$1
    local endpoint=$2
    local data=${3:-""}
    local expected_status=${4:-200}

    echo "🔍 Testing $method $endpoint"

    if [ -n "$data" ]; then
        if [ -n "$JWT_TOKEN" ]; then
            response=$(curl -s -w "%{http_code}" -X "$method" \
                -H "Authorization: Bearer $JWT_TOKEN" \
                -H "Content-Type: application/json" \
                -d "$data" \
                "$APP_URL$endpoint")
        else
            response=$(curl -s -w "%{http_code}" -X "$method" \
                -H "Content-Type: application/json" \
                -d "$data" \
                "$APP_URL$endpoint")
        fi
    else
        if [ -n "$JWT_TOKEN" ]; then
            response=$(curl -s -w "%{http_code}" -X "$method" \
                -H "Authorization: Bearer $JWT_TOKEN" \
                "$APP_URL$endpoint")
        else
            response=$(curl -s -w "%{http_code}" -X "$method" "$APP_URL$endpoint")
        fi
    fi

    status_code="${response: -3}"
    response_body="${response%???}"

    if [ "$status_code" -eq "$expected_status" ]; then
        echo "✅ $method $endpoint - Status: $status_code"
        return 0
    else
        echo "❌ $method $endpoint - Expected: $expected_status, Got: $status_code"
        echo "Response: $response_body"
        return 1
    fi
}

# Start cluster
echo "🚀 Starting Redis cluster..."
docker-compose -f $CLUSTER_COMPOSE_FILE up -d

# Wait for cluster to be ready
echo "⏳ Waiting for Redis cluster to initialize..."
sleep 20

# Wait for application to be ready
wait_for_service $APP_URL 60

# Test basic health endpoint
echo ""
echo "🏥 Testing health endpoint..."
test_api "GET" "/health"

# Test metrics endpoint
echo ""
echo "📊 Testing metrics endpoint..."
test_api "GET" "/metrics"

# Test Redis cluster functionality through app
echo ""
echo "🔧 Testing Redis cluster through application..."

# Generate a test JWT token (if jwt-gen exists)
if [ -f "./bin/jwt-gen" ]; then
    echo "🔑 Generating test JWT token..."
    JWT_TOKEN=$(./bin/jwt-gen -secret="$JWT_SECRET" -user="$JWT_USER")
    echo "Generated JWT: ${JWT_TOKEN:0:50}..."
    
    # Test widget creation (requires JWT)
    echo ""
    echo "📝 Testing widget creation..."
    widget_data='{
        "name": "Test Cluster Widget",
        "type": "lead-form",
        "enabled": true,
        "fields": {
            "name": {"type": "text", "required": true},
            "email": {"type": "email", "required": true}
        }
    }'
    
    create_response=$(curl -s -L -w "%{http_code}" -X POST \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$widget_data" \
        "$APP_URL/api/v1/widgets")
    
    status_code="${create_response: -3}"
    response_body="${create_response%???}"
    
    if [ "$status_code" -eq 201 ]; then
        echo "✅ Widget creation - Status: $status_code"
        
        # Extract widget ID from response
        widget_id=$(echo "$response_body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 || echo "")
        
        if [ -n "$widget_id" ]; then
            echo "📋 Created widget with ID: $widget_id"
            
            # Test widget submission
            echo ""
            echo "📤 Testing widget submission..."
            submission_data='{
                "data": {
                    "name": "Test User",
                    "email": "test@example.com"
                }
            }'
            
            test_api "POST" "/widgets/$widget_id/submit" "$submission_data" 201
            
            # Test widget events
            echo ""
            echo "Testing widget view event..."
            view_event='{"type": "view"}'
            test_api "POST" "/widgets/$widget_id/events" "$view_event" 204
            
            echo ""
            echo "Testing widget close event..."
            close_event='{"type": "close"}'
            test_api "POST" "/widgets/$widget_id/events" "$close_event" 204
            
            # Test widget stats
            echo ""
            echo "Testing widget statistics..."
            test_api "GET" "/api/v1/widgets/$widget_id/stats" "" 200
        else
            echo "⚠️ Could not extract widget ID from response"
        fi
    else
        echo "❌ Widget creation failed - Status: $status_code"
        echo "Response: $response_body"
    fi
else
    echo "⚠️ JWT generator not found, skipping authenticated tests"
fi

# Test Redis cluster directly
echo ""
echo "🔍 Testing Redis cluster directly..."
docker-compose -f $CLUSTER_COMPOSE_FILE exec -T redis-node-1 redis-cli cluster info | head -5

# Test data distribution
echo ""
echo "📊 Testing data distribution across cluster..."
for i in {1..3}; do
    echo -n "Node $i keys: "
    docker-compose -f $CLUSTER_COMPOSE_FILE exec -T redis-node-$i redis-cli dbsize 2>/dev/null || echo "0"
done

echo ""
echo "🎉 Redis Cluster Integration Test Complete!"
echo ""
echo "To view cluster status: ./redis-cluster.sh status"
echo "To view logs: docker-compose -f $CLUSTER_COMPOSE_FILE logs -f"
echo "To stop cluster: ./redis-cluster.sh stop"
