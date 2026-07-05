package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lihai1/stat-tree-server/internal/config"
	"github.com/lihai1/stat-tree-server/internal/startup"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server, err := startup.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("Servers started - gRPC on port %s, REST on port %s", cfg.Server.GRPCPort, cfg.Server.GatewayPort)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := server.Stop(); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
