# Nethouse Leads Service

A Go-based leads management service that handles forms, submissions, and real-time statistics using Redis Cluster as the primary storage.

## Features

- **Form Management**: Create, update, delete, and manage forms
- **Submission Handling**: Accept and store form submissions with TTL
- **Real-time Statistics**: Track views, submissions, and closes
- **JWT Authentication**: Secure API endpoints with JWT tokens
- **Rate Limiting**: IP-based and global rate limiting
- **Redis Cluster**: Scalable storage with Redis Cluster
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
docker-compose -f docker-compose.cluster.yml up -d
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

#### Create a form (Private endpoint - requires JWT):
```bash
curl -X POST http://localhost:8080/forms \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Contact Form",
    "type": "contact",
    "enabled": true,
    "fields": {
      "name": {"type": "text", "required": true},
      "email": {"type": "email", "required": true},
      "message": {"type": "textarea", "required": false}
    }
  }'
```

#### Submit to a form (Public endpoint):
```bash
curl -X POST http://localhost:8080/forms/FORM_ID/submit \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "name": "John Doe",
      "email": "john@example.com",
      "message": "Hello from the contact form!"
    }
  }'
```

#### Register form events (Public endpoint):
```bash
# Register a view event
curl -X POST http://localhost:8080/forms/FORM_ID/events \
  -H "Content-Type: application/json" \
  -d '{"type": "view"}'

# Register a close event
curl -X POST http://localhost:8080/forms/FORM_ID/events \
  -H "Content-Type: application/json" \
  -d '{"type": "close"}'
```

## API Endpoints

### Private Endpoints (Require JWT Authentication)

- `GET /forms` - List user's forms with pagination
- `POST /forms` - Create a new form
- `GET /forms/{id}` - Get form by ID
- `PUT /forms/{id}` - Update form
- `DELETE /forms/{id}` - Delete form
- `GET /forms/{id}/stats` - Get form statistics
- `GET /forms/{id}/submissions` - Get form submissions with pagination

### Public Endpoints

- `POST /forms/{id}/submit` - Submit data to a form
- `POST /forms/{id}/events` - Register form events (view, close)

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
# Single Redis instance
REDIS_ADDRESSES=redis:6379
# Redis cluster (comma-separated addresses)
REDIS_ADDRESSES=redis-node-1:6379,redis-node-2:6379,redis-node-3:6379,redis-node-4:6379,redis-node-5:6379,redis-node-6:6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key

# Rate Limiting
RATE_LIMIT_IP_PER_MINUTE=1
RATE_LIMIT_GLOBAL_PER_MINUTE=1000

# TTL Settings
TTL_FREE_DAYS=30
TTL_PRO_DAYS=365
```

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
- `make config-test` - Test configuration loading

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

The service uses Redis Cluster with the following key patterns:

- Forms: `form:{form_id}`
- Submissions: `submission:{form_id}:{submission_id}`
- User forms: `forms:{user_id}`
- Statistics: `form:{form_id}:stats`
- Rate limiting: `rate_limit:ip:{ip}:{window}`

## Security

- JWT token validation for private endpoints
- Rate limiting to prevent abuse
- Input validation for all requests
- Automatic TTL for submissions based on user plan

## License

See LICENSE file for details.