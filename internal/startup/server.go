package startup

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lihai1/stat-tree-server/internal/config"
	"github.com/lihai1/stat-tree-server/internal/database"
	"github.com/lihai1/stat-tree-server/internal/repository"
	"github.com/lihai1/stat-tree-server/internal/seeder"
	"github.com/lihai1/stat-tree-server/internal/server"
	"github.com/lihai1/stat-tree-server/internal/services"
)

// Server holds the server instances and dependencies
type Server struct {
	grpcServer    *server.GRPCServer
	gatewayServer *server.GatewayServer
	db            *database.Database
}

// NewServer initializes and returns a new Server instance
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize database
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		return nil, err
	}

	// Initialize repositories (available for future use)
	_ = repository.NewUserRepository(db.Pool)
	_ = repository.NewSavedFormRepository(db.Pool)

	// Initialize lottery result repository
	lotteryResultRepo := repository.NewLotteryResultRepository(db.Pool)

	log.Println("Repositories initialized")

	// Seed lottery data from pais.co.il on startup
	lotterySeeder := seeder.NewLotterySeeder(lotteryResultRepo)
	if err := lotterySeeder.SeedLotteryData(context.Background()); err != nil {
		log.Printf("Warning: Failed to seed lottery data: %v", err)
		// Continue startup even if seeding fails
	}

	// Initialize lottery service
	lotteryService := services.NewLotteryService()

	// Create gRPC server
	grpcServer := server.NewGRPCServer(cfg.Server.GRPCPort, lotteryService)

	// Create REST gateway server
	gatewayServer, err := server.NewGatewayServer(cfg.Server.GatewayPort, cfg.Server.GRPCPort, lotteryService)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Server{
		grpcServer:    grpcServer,
		gatewayServer: gatewayServer,
		db:            db,
	}, nil
}

// Start starts both gRPC and REST gateway servers
func (s *Server) Start() error {
	// Start gRPC server in goroutine
	errChan := make(chan error, 2)

	go func() {
		if err := s.grpcServer.Start(); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Start REST gateway in goroutine
	go func() {
		if err := s.gatewayServer.Start(); err != nil {
			errChan <- fmt.Errorf("REST gateway error: %w", err)
		}
	}()

	// Wait for any server to fail
	go func() {
		if err := <-errChan; err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully stops both servers
func (s *Server) Stop() error {
	log.Println("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.grpcServer.Stop()
	if err := s.gatewayServer.Stop(ctx); err != nil {
		// Ignore "http: Server closed" error as it's expected during graceful shutdown
		if err.Error() != "http: Server closed" {
			log.Printf("Gateway shutdown error: %v", err)
		}
	}

	// Close database connection
	if s.db != nil {
		s.db.Close()
	}

	log.Println("Servers stopped")
	return nil
}
