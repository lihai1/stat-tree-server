# Stat-Tree Server

Go backend server for the Statistiloto lottery analysis application using gRPC and REST API.

## Technology Stack

- Go 1.25+
- gRPC for internal service communication
- gRPC-Gateway for REST API
- Protocol Buffers for service definitions
- PostgreSQL with pgx driver
- Liquibase for database migrations
- JWT authentication
- Ginkgo/Gomega for testing
- Environment-based configuration

## Project Structure

```
stat-tree-server/
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── database/            # Database connection
│   ├── lottery-tree/        # Lottery tree data structure
│   ├── middleware/          # HTTP middleware (CORS, logging, auth)
│   ├── models/              # Data models
│   ├── repository/         # Data access layer (repository pattern)
│   ├── scraper/             # Web scraping for lottery data
│   ├── seeder/              # Database seeding
│   ├── server/              # gRPC and Gateway server setup
│   ├── services/            # Business logic services
│   └── startup/             # Application startup logic
├── db-migration/            # Liquibase database migrations
│   ├── db.changelog-master.yaml
│   ├── liquibase.properties
│   └── migrations/         # Individual migration files
├── pkg/
│   ├── clients/             # gRPC and HTTP clients
│   └── gen/                 # Generated protobuf files
├── proto/                   # Protocol Buffer definitions
│   ├── lottery.proto        # Lottery service definitions
│   └── third_party/         # Google API dependencies
├── tests/
│   └── integration/         # Integration tests
├── go.mod                   # Go module definition
├── go.sum                   # Go dependencies lock file
├── Makefile                 # Build and test automation
├── Dockerfile               # Docker container definition
├── Dockerfile.test          # Test container definition
├── docker-compose.yml       # Docker Compose for development
├── docker-compose-test.yml  # Docker Compose for testing
└── .env.example             # Environment variables template
```

## Quick Start

### Prerequisites

- Go 1.25 or higher
- Protocol Buffers compiler (protoc)
- PostgreSQL 15 or higher (or use Docker)

### Using Makefile

The project includes a Makefile for common operations:

```bash
# Install dependencies
make deps

# Generate protobuf files
make proto

# Build the application
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detection
make test-race

# Start PostgreSQL database
make postgres-db

# Run tests with PostgreSQL database
make compose-test

# Stop PostgreSQL database
make postgres-stop

# Run Liquibase migrations
make migrate-up

# Rollback Liquibase migrations
make migrate-down

# Check Liquibase migration status
make migrate-status

# Apply seed data
make db-seed

# Run the application
make run

# Clean build artifacts
make clean

# Run all checks (proto, build, test)
make all
```

### Manual Setup

1. Copy environment variables:
```bash
cp .env.example .env
```

2. Install dependencies:
```bash
go mod download
```

3. Generate protobuf files:
```bash
make proto
```

4. Run the server:
```bash
go run cmd/server/main.go
```

The server will start:
- gRPC server on `http://localhost:9090`
- REST API gateway on `http://localhost:8080`

### Running Tests

The project uses the Ginkgo testing framework with Gomega matchers:

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -v -cover

# Run with race detection
go test ./... -v -race
```

### Building for Production

```bash
go build -o stat-tree-server cmd/server/main.go
```

## API Endpoints

### REST API (via gRPC-Gateway)

- `GET /health` - Health check endpoint
- `POST /api/generate/form` - Generate lottery number combinations
- `POST /api/generate/statistics` - Get statistics on number pairs
- `POST /api/generate/analyze` - Analyze user-selected numbers

### gRPC Services

The server exposes gRPC services defined in `proto/lottery.proto`:
- `GenerateForm` - Generate lottery number combinations
- `GetStatistics` - Calculate statistics for number pairs
- `Analyze` - Analyze user-selected numbers against historical data

## Configuration

Configuration is managed through environment variables. See `.env.example` for all available options:

```bash
# Server Configuration
SERVER_PORT=8080              # REST API port
SERVER_HOST=0.0.0.0           # Server host
GRPC_PORT=9090                # gRPC server port
GATEWAY_PORT=8080             # Gateway port

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=statistiloto
DB_SSLMODE=disable

# JWT Configuration
JWT_SECRET=your-secret-key-change-this-in-production
```

## Architecture

The backend follows a clean architecture pattern with clear separation of concerns:

- **Services**: Business logic and gRPC service implementations
- **Handlers**: HTTP request/response handling (REST endpoints)
- **Middleware**: Cross-cutting concerns (CORS, logging, authentication)
- **Models**: Data structures and domain models
- **Repository**: Data access layer using repository pattern
- **Server**: gRPC and Gateway server setup
- **Config**: Configuration management
- **Database**: Database connection and query handling

The application is designed to be stateless and scalable:
- gRPC for efficient internal service communication
- REST API for external client access via gRPC-Gateway
- Connection pooling for database access
- JWT tokens for authentication
- Protocol Buffers for efficient serialization
- Liquibase for database schema management

## Database

### Schema

The application uses PostgreSQL with the following main tables:

- **users**: User accounts with authentication credentials
  - `id` (UUID, primary key)
  - `username` (unique)
  - `email` (unique)
  - `password_hash`
  - `created_at`, `updated_at`

- **saved_forms**: User-saved lottery form configurations
  - `id` (UUID, primary key)
  - `user_id` (foreign key to users)
  - `name` (form name/description)
  - `form_type` (lucky_numbers, statistical, etc.)
  - `numbers` (JSONB array)
  - `exclude_numbers` (JSONB array, optional)
  - `count` (integer, optional)
  - `analysis_type` (string, optional)
  - `generated_result` (JSONB, optional)
  - `created_at`, `updated_at`

### Migrations

Database migrations are managed using Liquibase:

```bash
# Run migrations
make migrate-up

# Rollback migrations
make migrate-down

# Check migration status
make migrate-status

# Apply seed data
make db-seed
```

Migrations are located in the `db-migration/` directory and are automatically applied when using docker-compose.

### Repository Pattern

The application uses the repository pattern for data access:

- **UserRepository**: CRUD operations for user accounts
- **SavedFormRepository**: CRUD operations for saved forms
- **LotteryResultRepository**: CRUD operations for lottery results

Repositories are initialized in `main.go` and can be injected into services when needed.

## Docker Deployment

### Individual Repository Testing

For testing the stat-tree-server independently with database:

```bash
# Start PostgreSQL and run migrations
docker-compose up -d

# The database will be available at localhost:5432
# Migrations will be applied automatically

# Stop services
docker-compose down
```

### Full Stack Deployment

Build and run with Docker:

```bash
# Build the image
docker build -t stat-tree-server .

# Run the container
docker run -p 8080:8080 -p 9090:9090 stat-tree-server
```

Or use docker-compose from the parent directory:

```bash
docker-compose up stat-tree-server
```

## Development

### Adding New gRPC Services

1. Define the service in `proto/lottery.proto`
2. Implement the service in `internal/services/`
3. Register the service in `internal/server/grpc.go`
4. Add gateway mappings in the proto file
5. Regenerate protobuf files: `make proto`

The protobuf files are generated in `pkg/gen/` using the Makefile proto target.

### Testing

Tests are written using the Ginkgo BDD framework with Gomega matchers. See existing test files in `internal/` for examples.

## Documentation

- [API Documentation](../docs/API.md)
- [Architecture Documentation](../docs/ARCHITECTURE.md)
