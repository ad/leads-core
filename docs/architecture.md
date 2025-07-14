# Architecture Documentation

## Overview

Nethouse Leads Service is a Go-based microservice designed to handle form management, submission processing, and real-time statistics. The service follows clean architecture principles and uses Redis Cluster as its primary storage.

## System Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │    │  Load       │    │  leads-core │
│ Application │◄──►│ Balancer    │◄──►│   Service   │
└─────────────┘    └─────────────┘    └─────────────┘
                                             │
                                             ▼
                                    ┌─────────────┐
                                    │ Redis       │
                                    │ Cluster     │
                                    └─────────────┘
```

## Component Architecture

### HTTP Layer
- **Handlers**: Process HTTP requests and responses
- **Middleware**: Authentication, rate limiting, logging
- **Routing**: URL routing and method handling

### Business Logic Layer
- **Services**: Core business logic and workflows
- **Models**: Data structures and validation
- **Auth**: JWT token validation and user context

### Data Layer
- **Storage**: Repository pattern for data access
- **Redis**: Key-value storage with cluster support

## Data Flow

### Private Endpoints (Authenticated)
```
Client Request → Auth Middleware → Rate Limiting → Handler → Service → Repository → Redis
```

### Public Endpoints
```
Client Request → Rate Limiting → Handler → Service → Repository → Redis
```

## Key Design Decisions

### 1. Redis as Primary Storage
- **Pros**: High performance, built-in TTL, clustering support
- **Cons**: Data persistence concerns, memory usage
- **Decision**: Suitable for leads data with TTL requirements

### 2. Repository Pattern
- **Benefits**: Testability, abstraction, dependency injection
- **Implementation**: Interface-based with Redis implementation

### 3. JWT Authentication
- **Stateless**: No server-side session storage
- **Claims**: User ID, plan information for TTL decisions

### 4. Rate Limiting Strategy
- **IP-based**: Prevent abuse from single sources
- **Global**: Protect overall system capacity
- **Redis-backed**: Distributed rate limiting

## Redis Data Model

### Key Patterns
```
# Forms
form:{form_id}                    # HASH - form metadata
forms:by_time                     # ZSET - forms by creation time
forms:{user_id}                   # SET - user's forms
forms:type:{type}                 # SET - forms by type
forms:enabled:{0|1}               # SET - forms by status

# Submissions
submission:{form_id}:{submission_id}  # HASH - submission data
form:{form_id}:submissions           # ZSET - form submissions by time

# Statistics
form:{form_id}:stats                 # HASH - aggregated stats
stats:form:{form_id}:views:{date}    # INCR - daily view counts

# Rate Limiting
rate_limit:ip:{ip}:{window}          # INCR - IP-based limits
rate_limit:global:{window}           # INCR - global limits
```

### TTL Strategy
- **Submissions**: Based on user plan (30 days free, 365 days pro)
- **Rate Limits**: 1-minute windows
- **Stats**: Daily stats kept for 30 days

## Security Model

### Authentication
- JWT tokens with HMAC-SHA256 signing
- Required claims: `user_id`, `exp`, `iat`
- Optional claims: `username`, `plan`

### Authorization
- Resource ownership validation
- User context propagation through request pipeline

### Rate Limiting
- IP-based: 1 request/minute default
- Global: 1000 requests/minute default
- Sliding window implementation

## Scalability Considerations

### Horizontal Scaling
- Stateless service design
- Redis Cluster for data layer scaling
- Load balancer distribution

### Performance
- Connection pooling to Redis
- Pipeline operations for bulk updates
- Efficient key design for range queries

### Monitoring
- Health checks for service and dependencies
- Structured logging for observability
- Metrics collection points

## Deployment Architecture

### Docker Compose (Development)
```
┌─────────────┐
│ leads-core  │
│  (port 8080)│
└─────────────┘
       │
       ▼
┌─────────────┐
│ Redis       │
│ Cluster     │
│ (6 nodes)   │
└─────────────┘
```

### Production Considerations
- Multi-instance deployment
- External Redis cluster
- Reverse proxy/load balancer
- Environment-specific configuration

## Error Handling Strategy

### HTTP Errors
- 400: Bad Request (validation errors)
- 401: Unauthorized (invalid JWT)
- 403: Forbidden (access denied)
- 404: Not Found (missing resources)
- 429: Too Many Requests (rate limiting)
- 500: Internal Server Error (system errors)

### Recovery Mechanisms
- Graceful degradation on Redis failures
- Connection retry logic
- Request timeout handling

## Future Enhancements

### Planned Features
1. Data export functionality (CSV, JSON)
2. Advanced analytics and reporting
3. Webhook notifications
4. Form templates and sharing
5. Multi-tenant improvements

### Scalability Improvements
1. Caching layer implementation
2. Message queue for async processing
3. Database integration for long-term storage
4. Horizontal auto-scaling

## Development Guidelines

### Code Organization
- Clean architecture layers
- Dependency injection
- Interface-based design
- Comprehensive testing

### Testing Strategy
- Unit tests for business logic
- Integration tests for repositories
- End-to-end tests for workflows
- Mock implementations for external dependencies
