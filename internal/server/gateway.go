package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/lihai1/stat-tree-server/internal/middleware"
	"github.com/lihai1/stat-tree-server/internal/services"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GatewayServer wraps the REST gateway server.
type GatewayServer struct {
	server   *http.Server
	port     string
	grpcConn *grpc.ClientConn
}

// NewGatewayServer creates a new REST gateway server instance. The auth
// middleware validates Keycloak JWTs (defense-in-depth) on protected
// endpoints; the /health endpoint is left open for liveness probes.
func NewGatewayServer(port string, grpcPort string, lotteryService *services.LotteryService, authMW *middleware.AuthMiddleware) (*GatewayServer, error) {
	ctx := context.Background()
	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
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

	grpcConn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%s", grpcPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	err = lotteryv1.RegisterLotteryServiceHandler(ctx, mux, grpcConn)
	if err != nil {
		grpcConn.Close()
		return nil, fmt.Errorf("failed to register lottery service handler: %w", err)
	}

	// Swagger handler (admin/debug only).
	mux.HandlePath("GET", "/swagger", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		http.ServeFile(w, r, "pkg/gen/lottery.swagger.json")
	})

	// Build the handler chain: CORS → JWT auth (except /health) → gateway mux.
	var protected http.Handler = mux
	if authMW != nil {
		protected = authMW.JWT(protected)
	}
	protected = corsMiddleware(protected)

	// Top-level router: /health is open; everything else is protected.
	rootMux := http.NewServeMux()
	rootMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	rootMux.Handle("/", protected)

	return &GatewayServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%s", port),
			Handler: rootMux,
		},
		port:     port,
		grpcConn: grpcConn,
	}, nil
}

// Start starts the REST gateway server.
func (g *GatewayServer) Start() error {
	log.Printf("Starting REST gateway server on port %s", g.port)
	return g.server.ListenAndServe()
}

// Stop gracefully stops the REST gateway server.
func (g *GatewayServer) Stop(ctx context.Context) error {
	log.Println("Stopping REST gateway server...")
	if g.grpcConn != nil {
		g.grpcConn.Close()
	}
	err := g.server.Shutdown(ctx)
	if err != nil && err.Error() != "http: Server closed" {
		return err
	}
	return nil
}

// corsMiddleware adds CORS support to the gateway. In production, CORS is
// primarily handled by Traefik; this is a fallback for direct access.
func corsMiddleware(h http.Handler) http.Handler {
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
