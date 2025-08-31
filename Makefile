.PHONY: build run test clean docker-build docker-run setup deps lint

# Build configuration
BINARY_NAME=tg-release-bot
BUILD_DIR=./bin
CMD_DIR=./cmd/bot

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	@go run $(CMD_DIR)/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f releases.db

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Lint code
lint:
	@echo "Running linter..."
	@golangci-lint run

# Setup development environment
setup: deps
	@echo "Setting up development environment..."
	@cp env.example .env || true
	@mkdir -p data
	@echo "Please edit .env file with your configuration"

# Docker commands
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(BINARY_NAME) .

docker-run: docker-build
	@echo "Running Docker container..."
	@docker-compose up -d

docker-logs:
	@docker-compose logs -f

docker-stop:
	@docker-compose down

# Development with live reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	@echo "Starting development server with live reload..."
	@air

# Database operations
db-reset:
	@echo "Resetting database..."
	@rm -f releases.db data/releases.db

# Help
help:
	@echo "Available commands:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  lint          - Run linter"
	@echo "  setup         - Setup development environment"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker Compose"
	@echo "  docker-logs   - Show Docker logs"
	@echo "  docker-stop   - Stop Docker containers"
	@echo "  dev           - Run with live reload"
	@echo "  db-reset      - Reset database"
	@echo "  help          - Show this help"
