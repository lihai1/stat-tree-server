.PHONY: all build test proto swagger clean run help postgres-db compose-test postgres-stop stop  migrate-up migrate-down migrate-status db-seed integration-test

# Variables
BINARY_NAME=statistiloto-backend
PROTO_DIR=proto
OUTPUT_DIR=pkg/gen
THIRD_PARTY_DIR=$(PROTO_DIR)/third_party

# Default target
all: proto build test

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/server

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	go run ./cmd/server

# Generate protobuf files
proto:
	@echo "Generating protobuf code..."
	@echo "Proto directory: $(PROTO_DIR)"
	@echo "Output directory: $(OUTPUT_DIR)"
	@mkdir -p $(OUTPUT_DIR)
	protoc \
	  --proto_path="$(PROTO_DIR)" \
	  --proto_path="$(THIRD_PARTY_DIR)" \
	  --go_out="$(OUTPUT_DIR)" \
	  --go_opt=paths=source_relative \
	  --go-grpc_out="$(OUTPUT_DIR)" \
	  --go-grpc_opt=paths=source_relative \
	  --grpc-gateway_out="$(OUTPUT_DIR)" \
	  --grpc-gateway_opt=paths=source_relative \
	  --grpc-gateway_opt=generate_unbound_methods=true \
	  --openapiv2_out="$(OUTPUT_DIR)" \
	  --openapiv2_opt=json_names_for_fields=false \
	  $(PROTO_DIR)/lottery.proto
	@echo "Protobuf code generation completed successfully!"

# Run tests using ginkgo framework
test:
	@echo "Running tests with ginkgo framework..."
	go test -v -cover ./...

# Run integration tests
integration-test:
	@echo "Running integration tests..."
	go test -v ./tests/integration/


# Start PostgreSQL database using docker-compose
postgres-db: stop
	@echo "Starting PostgreSQL database..."
	docker-compose up liquibase
# 	@echo "Waiting for PostgreSQL to be ready..."
# 	@sleep 5

# Run tests with PostgreSQL database using docker-compose-test
compose-test: stop
	@echo "Building test image..."
	docker build -f Dockerfile.test -t stat-tree-test:latest .
	@echo "Starting test environment..."
	docker-compose -f docker-compose-test.yml up liquibase test

# Stop all services and remove volumes
stop:
	@echo "Stopping all services and removing volumes..."
	docker-compose down -v
	docker-compose -f docker-compose-test.yml down -v

# Run Liquibase migrations up
migrate-up:
	@echo "Running Liquibase migrations..."
	docker-compose up liquibase

# Run Liquibase migrations down (rollback)
migrate-down:
	@echo "Rolling back Liquibase migrations..."
	docker-compose run --rm liquibase --defaults-file=/liquibase/changelog/liquibase.properties rollbackCount 1

# Check Liquibase migration status
migrate-status:
	@echo "Checking Liquibase migration status..."
	docker-compose run --rm liquibase --defaults-file=/liquibase/changelog/liquibase.properties status

# Apply seed data
db-seed: migrate-up
	@echo "Seed data applied via migrations"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	go clean
	rm -f $(BINARY_NAME) &
	rm -f *.out &
	rm -rf $(OUTPUT_DIR) &

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy


# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Run vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Help target
help:
	@echo "Available targets:"
	@echo "  all           - Run proto, build, and test"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  proto         - Generate protobuf files"
	@echo "  test          - Run tests with go test"
	@echo "  integration-test - Run integration tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  postgres-db   - Start PostgreSQL database using docker-compose"
	@echo "  compose-test  - Run tests with PostgreSQL database using docker-compose-test"
	@echo "  stop          - Stop all services and remove volumes"
	@echo "  migrate-up    - Run Liquibase migrations"
	@echo "  migrate-down  - Rollback Liquibase migrations"
	@echo "  migrate-status - Check Liquibase migration status"
	@echo "  db-seed       - Apply seed data via migrations"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  vet           - Run go vet"
	@echo "  help          - Show this help message"
