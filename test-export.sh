#!/bin/bash

# Test script for export functionality
# Make sure the server is running before executing this script

API_BASE="http://localhost:8080"
TEST_USER="export-test-user"
JWT_SECRET=development-jwt-secret-change-in-production
JWT_TOKEN=$(./bin/jwt-gen -secret="$JWT_SECRET" -user="$TEST_USER" -ttl="1h")

echo "Testing Export Functionality"
echo "==========================="

# Create a test widget first
echo "1. Creating a test widget..."
WIDGET_RESPONSE=$(curl -s -X POST "${API_BASE}/api/v1/widgets" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Export Test Widget",
    "type": "lead-form",
    "isVisible": true,
    "config": {
      "name": {"type": "text", "required": true},
      "email": {"type": "email", "required": true},
      "message": {"type": "textarea", "required": false}
    }
  }')

WIDGET_ID=$(echo $WIDGET_RESPONSE | jq -r '.data.id')
echo "Created widget with ID: $WIDGET_ID"

if [ "$WIDGET_ID" = "null" ]; then
    echo "Failed to create widget. Response: $WIDGET_RESPONSE"
    exit 1
fi

# Submit some test data
echo ""
echo "2. Submitting test data..."

curl -s -X POST "${API_BASE}/widgets/${WIDGET_ID}/submit" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "name": "John Doe",
      "email": "john@example.com",
      "message": "First test submission"
    }
  }' > /dev/null

sleep 1

curl -s -X POST "${API_BASE}/widgets/${WIDGET_ID}/submit" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "name": "Jane Smith",
      "email": "jane@example.com",
      "message": "Second test submission"
    }
  }' > /dev/null

sleep 1

curl -s -X POST "${API_BASE}/widgets/${WIDGET_ID}/submit" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "name": "Bob Johnson",
      "email": "bob@example.com"
    }
  }' > /dev/null

echo "Submitted 3 test submissions"

# Test exports
echo ""
echo "3. Testing exports..."

# Test JSON export
echo "Testing JSON export..."
curl -s -X GET "${API_BASE}/api/v1/widgets/${WIDGET_ID}/export?format=json" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -o "test_export.json"
echo "JSON export saved to test_export.json"

# Test CSV export
echo "Testing CSV export..."
curl -s -X GET "${API_BASE}/api/v1/widgets/${WIDGET_ID}/export?format=csv" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -o "test_export.csv"
echo "CSV export saved to test_export.csv"

# Test XLSX export
echo "Testing XLSX export..."
curl -s -X GET "${API_BASE}/api/v1/widgets/${WIDGET_ID}/export?format=xlsx" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -o "test_export.xlsx"
echo "XLSX export saved to test_export.xlsx"

# Test date range export
echo "Testing date range export..."
TODAY=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
YESTERDAY=$(date -u -d "yesterday" +"%Y-%m-%dT00:00:00Z" 2>/dev/null || date -u -v-1d +"%Y-%m-%dT00:00:00Z")

curl -s -X GET "${API_BASE}/api/v1/widgets/${WIDGET_ID}/export?format=csv&from=${YESTERDAY}&to=${TODAY}" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -o "test_export_range.csv"
echo "Date range CSV export saved to test_export_range.csv"

# Show file sizes
echo ""
echo "4. Export file information:"
echo "=========================="
ls -la test_export.*

# Show sample content
echo ""
echo "5. Sample CSV content:"
echo "====================="
head -5 test_export.csv

echo ""
echo "6. Sample JSON content:"
echo "======================"
head -10 test_export.json

# Clean up test widget
echo ""
echo "7. Cleaning up..."
curl -s -X DELETE "${API_BASE}/api/v1/widgets/${WIDGET_ID}" \
  -H "Authorization: Bearer ${JWT_TOKEN}" > /dev/null
echo "Test widget deleted"

# Clean up generated export files
echo "Cleaning up generated export files..."
rm -f test_export*
echo "Export files cleaned up"

echo ""
echo "Export functionality test completed!"
echo "Check the generated files: test_export.json, test_export.csv, test_export.xlsx"
