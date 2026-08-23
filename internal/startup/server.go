package startup

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lihai1/stat-tree-server/internal/config"
	"github.com/lihai1/stat-tree-server/internal/database"
	"github.com/lihai1/stat-tree-server/internal/middleware"
	"github.com/lihai1/stat-tree-server/internal/repository"
	"github.com/lihai1/stat-tree-server/internal/scraper"
	"github.com/lihai1/stat-tree-server/internal/seeder"
	"github.com/lihai1/stat-tree-server/internal/server"
	"github.com/lihai1/stat-tree-server/internal/services"
)

// Server holds the server instances and dependencies.
// The Go service is stateless beyond the lottery_results table: it owns no
// user data and no saved forms (those moved to the Java BFF + Keycloak).
type Server struct {
	grpcServer    *server.GRPCServer
	gatewayServer *server.GatewayServer
	db            *database.Database
	scraperStop   chan struct{}
}

// NewServer initializes and returns a new Server instance.
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize database (search_path is set to the lottery schema).
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		return nil, err
	}

	// Initialize the only repository this service owns.
	lotteryResultRepo := repository.NewLotteryResultRepository(db.Pool)
	log.Println("Repositories initialized (lottery_results only)")

	// Seed lottery data on first boot if enabled and the table is empty.
	if cfg.Scraper.SeedOnBoot {
		lotterySeeder := seeder.NewLotterySeeder(lotteryResultRepo)
		if err := lotterySeeder.SeedLotteryData(context.Background()); err != nil {
			log.Printf("Warning: Failed to seed lottery data: %v", err)
			// Continue startup even if seeding fails — the scraper cron
			// will retry.
		}
	}

	// Initialize lottery service (algorithm + tree), wired to the
	// lottery_results repository so each RPC can load the historical archive.
	lotteryService := services.NewLotteryService(lotteryResultRepo)

	// Create gRPC server.
	grpcServer := server.NewGRPCServer(cfg.Server.GRPCPort, lotteryService)

	// Create the Keycloak JWT auth middleware (defense-in-depth).
	authMW := middleware.NewAuthMiddleware(cfg.Auth)

	// Create REST gateway server with auth middleware.
	gatewayServer, err := server.NewGatewayServer(cfg.Server.GatewayPort, cfg.Server.GRPCPort, lotteryService, authMW)
	if err != nil {
		db.Close()
		return nil, err
	}

	s := &Server{
		grpcServer:    grpcServer,
		gatewayServer: gatewayServer,
		db:            db,
		scraperStop:   make(chan struct{}),
	}

	// Start the scheduled scraper to refresh lottery_results from pais.co.il.
	if cfg.Scraper.Cron != "" {
		s.startScraperScheduler(cfg.Scraper.Cron, lotteryResultRepo)
	} else {
		log.Println("Scraper cron schedule is empty; skipping scheduled refresh")
	}

	return s, nil
}

// startScraperScheduler runs a ticker that refreshes lottery_results on the
// configured cron schedule. A simple daily/hourly ticker is used here; for
// arbitrary cron expressions a library like robfig/cron would be needed.
func (s *Server) startScraperScheduler(cronSpec string, repo *repository.LotteryResultRepository) {
	interval := parseCronAsInterval(cronSpec)
	if interval <= 0 {
		log.Printf("Scraper schedule %q could not be parsed; skipping", cronSpec)
		return
	}
	log.Printf("Starting scheduled scraper every %s", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		scraperClient := scraper.NewPaisScraper()
		refresh := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			results, err := scraperClient.FetchLotteryData()
			if err != nil {
				log.Printf("Scraper fetch failed: %v", err)
				return
			}
			if err := repo.CreateBatch(ctx, results); err != nil {
				log.Printf("Scraper persist failed: %v", err)
				return
			}
			log.Printf("Scraper refreshed %d lottery results", len(results))
		}

		for {
			select {
			case <-ticker.C:
				refresh()
			case <-s.scraperStop:
				return
			}
		}
	}()
}

// parseCronAsInterval converts a simple cron spec into a refresh interval.
// Supports the default "0 3 * * *" (daily) and hourly "0 * * * *". For more
// complex schedules, integrate robfig/cron.
func parseCronAsInterval(cronSpec string) time.Duration {
	switch cronSpec {
	case "0 3 * * *": // daily at 03:00
		return 24 * time.Hour
	case "0 * * * *": // hourly
		return time.Hour
	case "*/15 * * * *": // every 15 minutes
		return 15 * time.Minute
	default:
		// Fallback: treat unknown specs as daily.
		return 24 * time.Hour
	}
}

// Start starts both gRPC and REST gateway servers.
func (s *Server) Start() error {
	errChan := make(chan error, 2)

	go func() {
		if err := s.grpcServer.Start(); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	go func() {
		if err := s.gatewayServer.Start(); err != nil {
			errChan <- fmt.Errorf("REST gateway error: %w", err)
		}
	}()

	go func() {
		if err := <-errChan; err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return nil
}

// Stop gracefully stops both servers and the scraper.
func (s *Server) Stop() error {
	log.Println("Shutting down servers...")

	// Stop the scraper goroutine.
	if s.scraperStop != nil {
		close(s.scraperStop)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.grpcServer.Stop()
	if err := s.gatewayServer.Stop(ctx); err != nil {
		if err.Error() != "http: Server closed" {
			log.Printf("Gateway shutdown error: %v", err)
		}
	}

	if s.db != nil {
		s.db.Close()
	}

	log.Println("Servers stopped")
	return nil
}
