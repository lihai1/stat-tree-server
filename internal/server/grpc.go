package server

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/lihai1/stat-tree-server/internal/middleware"
	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"

	"google.golang.org/grpc"
)

// GRPCServer wraps the gRPC server
type GRPCServer struct {
	server *grpc.Server
	port   string
}

// NewGRPCServer creates a new gRPC server instance. The authMW interceptor
// validates JWTs on every unary call, reusing the same AuthMiddleware (and
// JWKS cache) as the HTTP REST gateway.
func NewGRPCServer(port string, lotteryService *services.LotteryService, authMW *middleware.AuthMiddleware) *GRPCServer {
	opts := []grpc.ServerOption{}
	if authMW != nil {
		opts = append(opts, grpc.UnaryInterceptor(authMW.GRPCUnaryInterceptor()))
	}
	s := grpc.NewServer(opts...)
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

	slog.Info("starting gRPC server", "port", g.port)
	return g.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (g *GRPCServer) Stop() {
	slog.Info("stopping gRPC server...")
	g.server.GracefulStop()
}
