.PHONY: build run stop test clean logs help

# Default target
.DEFAULT_GOAL := help

# Build the Go application
build: ## Build the Go application
	@echo "Building Go application..."
	go build -o bin/leads-core cmd/server/main.go

# Build Docker image
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t leads-core:latest .

# Run with docker-compose
run: ## Start all services with docker-compose (single Redis)
	@echo "Starting services with docker-compose..."
	docker-compose up -d --build

# Run with Redis cluster
run-cluster: ## Start all services with Redis cluster
	@echo "Starting services with Redis cluster..."
	docker-compose -f docker-compose.cluster.yml up -d --build

# Stop all services
stop: ## Stop all services (single Redis)
	@echo "Stopping services..."
	docker-compose down

# Stop Redis cluster
stop-cluster: ## Stop Redis cluster services
	@echo "Stopping Redis cluster services..."
	docker-compose -f docker-compose.cluster.yml down

# Run tests
test: ## Run Go tests
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run tests with race check
test-race: ## Run tests with race check
	@echo "Running tests with race check..."
	go test -v -race ./...

# Clean up
clean: ## Clean up docker containers, images, and volumes (single Redis)
	@echo "Cleaning up..."
	docker-compose down -v --rmi all --remove-orphans
	docker system prune -f

# Clean up Redis cluster
clean-cluster: ## Clean up Redis cluster containers, images, and volumes
	@echo "Cleaning up Redis cluster..."
	docker-compose -f docker-compose.cluster.yml down -v --rmi all --remove-orphans
	docker system prune -f

# Show logs
logs: ## Show docker-compose logs (single Redis)
	docker-compose logs -f

# Show cluster logs
logs-cluster: ## Show Redis cluster logs
	docker-compose -f docker-compose.cluster.yml logs -f

# Show logs for specific service
logs-app: ## Show logs for leads-core service
	docker-compose logs -f leads-core

logs-redis: ## Show logs for redis services
	docker-compose logs -f redis-node-1 redis-node-2 redis-node-3

# Redis cluster management
cluster-start: ## Start Redis cluster
	@echo "Starting Redis cluster..."
	./redis-cluster.sh start

cluster-stop: ## Stop Redis cluster
	@echo "Stopping Redis cluster..."
	./redis-cluster.sh stop

cluster-status: ## Show Redis cluster status
	@echo "Redis cluster status:"
	./redis-cluster.sh status

cluster-test: ## Test Redis cluster functionality
	@echo "Testing Redis cluster..."
	./redis-cluster.sh test

cluster-clean: ## Clean Redis cluster data
	@echo "Cleaning Redis cluster data..."
	./redis-cluster.sh clean

cluster-integration-test: ## Run integration test with Redis cluster
	@echo "Running Redis cluster integration test..."
	./test-cluster.sh

# Development mode (run locally)
dev: ## Run application locally (requires Redis running, automatically loads .env)
	@echo "Running application in development mode..."
	@echo "Note: .env file will be automatically loaded if present"
	go run cmd/server/main.go

# Setup development environment
setup-dev: ## Setup development environment (.env file)
	@echo "Setting up development environment..."
	@if [ ! -f .env ]; then \
		cp configs/.env.example .env; \
		echo ".env file created from example. Please edit as needed."; \
	else \
		echo ".env file already exists."; \
	fi

# Test configuration loading
config-test: ## Test configuration loading (including .env)
	@echo "Testing configuration loading..."
	go run cmd/config-test/main.go

# Install dependencies
deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Format code
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
lint: ## Lint Go code
	@echo "Linting code..."
	golangci-lint run

# Show help
help: ## Show this help message
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
