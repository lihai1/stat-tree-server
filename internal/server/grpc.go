package server

import (
	"fmt"
	"log"
	"net"
	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"

	"google.golang.org/grpc"
)

// GRPCServer wraps the gRPC server
type GRPCServer struct {
	server *grpc.Server
	port   string
}

// NewGRPCServer creates a new gRPC server instance
func NewGRPCServer(port string, lotteryService *services.LotteryService) *GRPCServer {
	s := grpc.NewServer()
	lotteryv1.RegisterLotteryServiceServer(s, lotteryService)

	return &GRPCServer{
		server: s,
		port:   port,
	}
}

// Start starts the gRPC server
func (g *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", g.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", g.port, err)
	}

	log.Printf("Starting gRPC server on port %s", g.port)
	return g.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (g *GRPCServer) Stop() {
	log.Println("Stopping gRPC server...")
	g.server.GracefulStop()
}
