# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install protoc and required plugins
RUN apk add --no-cache protobuf-dev curl git make

# Install protoc-gen-go
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Install protoc-gen-go-grpc
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install protoc-gen-grpc-gateway
RUN go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# Install protoc-gen-openapiv2 (for Swagger/OpenAPI generation)
RUN go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Regenerate protobuf files from the shared proto contract.
RUN make proto

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o statistiloto-server cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/statistiloto-server .

# Copy .env file if it exists (for local development)
COPY --from=builder /app/.env.example .env.example

# Expose ports for REST gateway and gRPC
EXPOSE 8080 9090

# Run the application
CMD ["./statistiloto-server"]
