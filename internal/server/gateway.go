package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GatewayServer wraps the REST gateway server
type GatewayServer struct {
	mux      *runtime.ServeMux
	server   *http.Server
	port     string
	grpcConn *grpc.ClientConn
}

// NewGatewayServer creates a new REST gateway server instance
func NewGatewayServer(port string, grpcPort string, lotteryService *services.LotteryService) (*GatewayServer, error) {
	ctx := context.Background()
	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			// Allow common headers
			allowedHeaders := map[string]bool{
				"authorization": true,
				"content-type":  true,
				"accept":        true,
				"x-csrf-token":  true,
				"x-request-id":  true,
			}
			return key, allowedHeaders[key]
		}),
	)

	// Connect to gRPC server
	grpcConn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%s", grpcPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	// Register the lottery service
	err = lotteryv1.RegisterLotteryServiceHandler(ctx, mux, grpcConn)
	if err != nil {
		grpcConn.Close()
		return nil, fmt.Errorf("failed to register lottery service handler: %w", err)
	}

	// Register swagger handler
	mux.HandlePath("GET", "/swagger", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		http.ServeFile(w, r, "pkg/gen/lottery.swagger.json")
	})

	return &GatewayServer{
		mux:      mux,
		port:     port,
		grpcConn: grpcConn,
	}, nil
}

// Start starts the REST gateway server
func (g *GatewayServer) Start() error {
	g.server = &http.Server{
		Addr:    fmt.Sprintf(":%s", g.port),
		Handler: g.corsMiddleware(g.mux),
	}

	log.Printf("Starting REST gateway server on port %s", g.port)
	return g.server.ListenAndServe()
}

// Stop gracefully stops the REST gateway server
func (g *GatewayServer) Stop(ctx context.Context) error {
	log.Println("Stopping REST gateway server...")
	if g.grpcConn != nil {
		g.grpcConn.Close()
	}
	err := g.server.Shutdown(ctx)
	// Ignore "http: Server closed" error as it's expected during graceful shutdown
	if err != nil && err.Error() != "http: Server closed" {
		return err
	}
	return nil
}

// corsMiddleware adds CORS support to the gateway
func (g *GatewayServer) corsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}
