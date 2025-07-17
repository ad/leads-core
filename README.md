# Nethouse Leads Service

A Go-based leads management service that handles widgets, submissions, and real-time statistics using Redis Cluster as the primary storage.

## Features

- **Widget Management**: Create, update, delete, and manage widgets
- **Submission Handling**: Accept and store widget submissions with TTL
- **Real-time Statistics**: Track views, submissions, and closes
- **JWT Authentication**: Secure API endpoints with JWT tokens
- **Rate Limiting**: IP-based and global rate limiting
- **Redis Storage**: Flexible Redis configuration with three options:
  - External Redis instance
  - Redis Cluster for high availability
  - **Embedded Redis server** (Redka) for simplified deployment
- **Docker Support**: Full containerization with docker-compose

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for local development)

### Run with Docker

1. Clone the repository:
```bash
git clone https://github.com/ad/leads-core.git
cd leads-core
```

2. Start all services:

**Option A: Single Redis instance (for development):**
```bash
make run
# или
docker-compose up -d
```

**Option B: Redis cluster (for production-like testing):**
```bash
./redis-cluster.sh start
# или
docker-compose -f docker-compose.cluster.yml up --build -d
```

**Option C: Embedded Redis (no external dependencies):**
```bash
# Set REDIS_ADDRESSES=redka in .env file
echo "REDIS_ADDRESSES=redka" >> .env

# Run the application
go run ./cmd/server/
# or with Docker
docker-compose up -d leads-server
```

3. Check service health:
```bash
curl http://localhost:8080/health
```

### Redis Cluster Management

For Redis cluster deployments, use the provided management script:

```bash
# Start Redis cluster
./redis-cluster.sh start

# Check cluster status
./redis-cluster.sh status

# Test cluster functionality
./redis-cluster.sh test

# View cluster logs
./redis-cluster.sh logs

# Stop cluster
./redis-cluster.sh stop

# Clean all cluster data
./redis-cluster.sh clean
```

### API Examples

#### Create a widget (Private endpoint - requires JWT):
```bash
curl -X POST http://localhost:8080/widgets \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Contact Widget",
    "type": "lead-form",
    "enabled": true,
    "fields": {
      "name": {"type": "text", "required": true},
      "email": {"type": "email", "required": true},
      "message": {"type": "textarea", "required": false}
    }
  }'
```

#### Submit to a widget (Public endpoint):
```bash
curl -X POST http://localhost:8080/widgets/550e8400-e29b-41d4-a716-446655440000/submit \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "name": "John Doe",
      "email": "john@example.com",
      "message": "Hello from the contact widget!"
    }
  }'
```

#### Register widget events (Public endpoint):
```bash
# Register a view event
curl -X POST http://localhost:8080/widgets/550e8400-e29b-41d4-a716-446655440000/events \
  -H "Content-Type: application/json" \
  -d '{"type": "view"}'

# Register a close event
curl -X POST http://localhost:8080/widgets/550e8400-e29b-41d4-a716-446655440000/events \
  -H "Content-Type: application/json" \
  -d '{"type": "close"}'
```

## Export Functionality

The service provides powerful export capabilities for widget submissions in multiple formats:

### Supported Formats

- **JSON**: Structured data with metadata, perfect for API integrations
- **CSV**: Comma-separated values, ideal for spreadsheet applications
- **XLSX**: Microsoft Excel format with styling and auto-fitting columns

### Export Features

- **Flexible Date Ranges**: Export data from specific time periods
- **Dynamic Field Detection**: Automatically detects all fields from submissions
- **Secure Access**: JWT authentication required for all exports
- **Filename Generation**: Auto-generates descriptive filenames with timestamps
- **Large Dataset Support**: Handles thousands of submissions efficiently

### Export API Parameters

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `format` | string | Export format: `json`, `csv`, `xlsx` | `?format=csv` |
| `from` | string | Start date (RFC3339) | `?from=2024-01-01T00:00:00Z` |
| `to` | string | End date (RFC3339) | `?to=2024-12-31T23:59:59Z` |

### Export Examples

#### Quick Export (All Data)
```bash
# Export all submissions as CSV
curl -X GET "http://localhost:8080/widgets/{widget-id}/export?format=csv" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -o submissions.csv
```

#### Date Range Export
```bash
# Export data from specific date range
curl -X GET "http://localhost:8080/widgets/{widget-id}/export?format=xlsx&from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -o submissions_2024.xlsx
```

#### JSON Export with Metadata
```bash
# Export as JSON with full metadata
curl -X GET "http://localhost:8080/widgets/{widget-id}/export?format=json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -o submissions.json
```

### JSON Export Structure

```json
{
  "widget": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Contact Widget",
    "type": "lead-form"
  },
  "exported_at": "2024-01-15T10:30:00Z",
  "total_count": 150,
  "submissions": [
    {
      "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "widget_id": "550e8400-e29b-41d4-a716-446655440000",
      "data": {
        "name": "John Doe",
        "email": "john@example.com",
        "message": "Contact form submission"
      },
      "created_at": "2024-01-15T09:15:30Z"
    }
  ]
}
```

### CSV Export Features

- **Dynamic Headers**: Automatically generates headers from all detected fields
- **Missing Field Handling**: Empty values for missing fields in submissions
- **Proper Escaping**: Handles commas, quotes, and newlines in data
- **UTF-8 Encoding**: Supports international characters

### Excel (XLSX) Export Features

- **Styled Headers**: Bold headers with background color
- **Auto-fit Columns**: Columns automatically sized for content
- **Proper Data Types**: Numbers, dates, and text formatted correctly
- **Large Dataset Support**: Handles thousands of rows efficiently

### Use Cases

1. **CRM Integration**: Export submissions for import into CRM systems
2. **Data Analysis**: Download data for analysis in Excel or Google Sheets
3. **Backup & Archive**: Regular data backups in multiple formats
4. **Reporting**: Generate reports for specific time periods
5. **Migration**: Export data for migration to other systems
```

## API Endpoints

### Private Endpoints (Require JWT Authentication)

- `GET /widgets` - List user's widgets with pagination
- `POST /widgets` - Create a new widget
- `GET /widgets/{id}` - Get widget by ID
- `PUT /widgets/{id}` - Update widget
- `DELETE /widgets/{id}` - Delete widget
- `GET /widgets/{id}/stats` - Get widget statistics
- `GET /widgets/{id}/submissions` - Get widget submissions with pagination
- `GET /widgets/{id}/export` - Export widget submissions in various formats

### Public Endpoints

- `POST /widgets/{id}/submit` - Submit data to a widget
- `POST /widgets/{id}/events` - Register widget events (view, close)

### System Endpoints

- `GET /health` - Service health check

## Configuration

Configuration is done via environment variables:

```bash
# Server Configuration
SERVER_PORT=8080
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s

# Redis Configuration  
# External Redis instance
REDIS_ADDRESSES=redis:6379
# External Redis cluster (comma-separated addresses)
REDIS_ADDRESSES=redis-node-1:6379,redis-node-2:6379,redis-node-3:6379,redis-node-4:6379,redis-node-5:6379,redis-node-6:6379
# Embedded Redis server (Redka)
REDIS_ADDRESSES=redka
REDIS_PASSWORD=
REDIS_DB=0

# Embedded Redis Configuration (only used when REDIS_ADDRESSES=redka)
REDKA_PORT=6379          # Port for embedded Redis server
REDKA_DB_PATH=file:redka.db  # Database file path (:memory: for in-memory)

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key

# Rate Limiting
RATE_LIMIT_IP_PER_MINUTE=1
RATE_LIMIT_GLOBAL_PER_MINUTE=1000

# TTL Settings for Submissions
TTL_FREE_DAYS=30          # Free plan: submissions expire after 30 days
TTL_PRO_DAYS=365          # Pro plan: submissions expire after 365 days
```

**Note on TTL Settings:**
- TTL applies only to submission data (`{widget_id}:submission:{submission_id}`)
- Widget data, statistics, and indexes persist permanently until manually deleted
- Daily view statistics have fixed 30-day TTL regardless of user plan
- Rate limiting keys use 1-minute TTL for sliding window implementation

## Development

### Local Development

1. Copy environment configuration:
```bash
cp configs/.env.example .env
# Edit .env with your local settings
```

2. Start Redis cluster with docker-compose:
```bash
docker-compose up redis-node-1 redis-node-2 redis-node-3 redis-node-4 redis-node-5 redis-node-6 redis-cluster-init
```

3. Run the application (will automatically load .env):
```bash
make dev
```

**Note:** The application automatically loads environment variables from `.env` file in the project root. The autoload will search for `.env` files in the following order:
- `.env.local` (loaded first, highest priority)
- `.env` (loaded second)
- Environment variables set in the system (lowest priority)

This makes it easy to override settings for different environments.

### Available Make Commands

- `make run` - Start all services with docker-compose
- `make stop` - Stop all services
- `make build` - Build the Go application
- `make test` - Run tests
- `make clean` - Clean up Docker containers and images
- `make logs` - Show logs
- `make dev` - Run application locally
- `make setup-dev` - Setup development environment (.env file)

### Testing Export Functionality

You can test the export functionality using the provided test script:

```bash
# Make sure the server is running first
make dev

# In another terminal, run the export test
./test-export.sh
```

This script will:
1. Create a test widget
2. Submit sample data
3. Test all export formats (JSON, CSV, XLSX)
4. Test date range filtering
5. Show file information and sample content
6. Clean up test data

### Testing

Run tests:
```bash
make test
```

Run tests with coverage:
```bash
make test-coverage
```

## Architecture

The service follows a clean architecture pattern:

```
cmd/server/main.go              # Application entry point
internal/
  ├── handlers/                 # HTTP request handlers
  ├── models/                   # Data structures
  ├── services/                 # Business logic
  ├── storage/                  # Redis operations
  ├── auth/                     # JWT authentication
  ├── config/                   # Configuration management
  └── middleware/               # HTTP middleware
```

## Data Storage

The service uses Redis with the following key patterns and TTL policies:

### Key Patterns with Hash Tags (for Redis Cluster compatibility)
- **Widgets**: `{widget_id}:widget` - Widget data (HASH)
- **Submissions**: `{widget_id}:submission:{submission_id}` - Submission data (HASH)
- **Widget Submissions Index**: `{widget_id}:submissions` - Widget submissions sorted by timestamp (ZSET)
- **Widget Statistics**: `{widget_id}:stats` - Widget stats (views, submits, closes) (HASH)
- **Daily Views**: `{widget_id}:views:{YYYY-MM-DD}` - Daily view counts (INCR)
- **User Widgets**: `{user_id}:user:widgets` - User's widgets index (SET)

### Global Indexes (without hash tags)
- **Widgets by Time**: `widgets:by_time` - All widgets sorted by creation time (ZSET)
- **Widgets by Type**: `widgets:type:{type}` - Widgets grouped by type (SET)
- **Widgets by Status**: `widgets:enabled:{0|1}` - Widgets grouped by enabled status (SET)

### Rate Limiting Keys
- **IP Rate Limit**: `rate_limit:{window}:ip:{ip}` - IP-based rate limiting (INCR)
- **Global Rate Limit**: `rate_limit:{window}:global` - Global rate limiting (INCR)

### ID Generation Strategy

**Widget IDs**: Generated using **UUID v5** with user_id as namespace
- Format: Standard UUID v5 (e.g., `550e8400-e29b-41d4-a716-446655440000`)
- Namespace: SHA-1 hash of user_id
- Name: `widget_{timestamp_nanoseconds}`
- **Benefits**: 
  - Deterministic (reproducible for debugging)
  - Logically grouped by user
  - Cryptographically secure (SHA-1 hash)
  - Guaranteed uniqueness within user namespace

**Submission IDs**: Generated using **UUID v5** with widget_id as namespace
- Format: Standard UUID v5
- Namespace: SHA-1 hash of widget_id  
- Name: `submission_{timestamp_nanoseconds}`
- **Benefits**:
  - Logically grouped by widget
  - Deterministic for debugging
  - Guaranteed uniqueness within widget namespace

### TTL (Time To Live) Policies

#### Keys with Automatic TTL:
- **Submissions**: TTL based on user plan
  - Free plan: 30 days (TTL_FREE_DAYS)
  - Pro plan: 365 days (TTL_PRO_DAYS)
- **Daily Views**: 30 days (fixed) - Daily statistics cleanup
- **Rate Limiting Keys**: 1 minute (sliding window)

#### Keys without TTL (persistent):
- **Widgets**: No TTL - persist until manually deleted
- **Widget Statistics**: No TTL - persist until widget is deleted
- **User Widgets Index**: No TTL - persist until manually cleaned
- **Global Indexes**: No TTL - persist until manually cleaned
- **Widget Submissions Index**: No TTL - persist until widget is deleted (but individual submissions may expire)

#### TTL Update Behavior:
- When user upgrades/downgrades plan, all their submissions get updated TTL
- New submissions inherit TTL based on current user plan
- TTL can be manually updated for specific users via admin API

### ID Format Examples

**Widget ID**: `550e8400-e29b-41d4-a716-446655440000`
- Generated using UUID v5 with user_id namespace
- Deterministic: same user + timestamp = same widget ID
- Safe: Cannot guess other users' widget IDs

**Submission ID**: `6ba7b810-9dad-11d1-80b4-00c04fd430c8`  
- Generated using UUID v5 with widget_id namespace
- Deterministic: same widget + timestamp = same submission ID
- Scoped: Cannot access submissions from other widgets

## Security

- **JWT token validation** for private endpoints
- **Rate limiting** to prevent abuse  
- **Input validation** for all requests
- **Automatic TTL** for submissions based on user plan
- **Secure ID generation**: UUID v5 with namespace-based deterministic generation
  - Widget IDs: Cannot be guessed or enumerated by attackers
  - Submission IDs: Scoped to specific widgets, preventing cross-widget access
  - SHA-1 hashing ensures cryptographic security while maintaining reproducibility

## Redis Configuration Options

The service supports three Redis deployment modes:

### 1. External Redis Instance
For development and small deployments:
```bash
REDIS_ADDRESSES=localhost:6379
```

### 2. Redis Cluster
For production and high-availability setups:
```bash
REDIS_ADDRESSES=node1:6379,node2:6379,node3:6379,node4:6379,node5:6379,node6:6379
```

### 3. Embedded Redis (Redka)
For simplified deployment without external Redis dependencies:
```bash
REDIS_ADDRESSES=redka
REDKA_PORT=6379
REDKA_DB_PATH=file:redka.db    # or :memory: for in-memory storage
```
