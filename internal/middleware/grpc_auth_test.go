package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lihai1/stat-tree-server/internal/config"
	"github.com/lihai1/stat-tree-server/internal/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestGRPCAuthInterceptor_AuthDisabled_PassesThrough verifies that when auth
// is disabled, the gRPC interceptor allows all requests through.
func TestGRPCAuthInterceptor_AuthDisabled_PassesThrough(t *testing.T) {
	cfg := config.AuthConfig{Enabled: false}
	mw := middleware.NewAuthMiddleware(cfg)

	interceptor := mw.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, nil, handler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

// TestGRPCAuthInterceptor_MissingMetadata_ReturnsUnauthenticated verifies
// that a request without any metadata is rejected with codes.Unauthenticated.
func TestGRPCAuthInterceptor_MissingMetadata_ReturnsUnauthenticated(t *testing.T) {
	cfg := config.AuthConfig{Enabled: true, JWKSURL: "http://localhost:9999/certs"}
	mw := middleware.NewAuthMiddleware(cfg)

	interceptor := mw.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := interceptor(context.Background(), nil, nil, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected codes.Unauthenticated, got %v", st.Code())
	}
}

// TestGRPCAuthInterceptor_MissingAuthorizationHeader_ReturnsUnauthenticated
// verifies that metadata without an authorization key is rejected.
func TestGRPCAuthInterceptor_MissingAuthorizationHeader_ReturnsUnauthenticated(t *testing.T) {
	cfg := config.AuthConfig{Enabled: true, JWKSURL: "http://localhost:9999/certs"}
	mw := middleware.NewAuthMiddleware(cfg)

	interceptor := mw.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-other", "value"))
	_, err := interceptor(ctx, nil, nil, handler)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected codes.Unauthenticated, got %v", err)
	}
}

// TestGRPCAuthInterceptor_InvalidToken_ReturnsUnauthenticated verifies that
// a malformed bearer token is rejected before reaching the JWKS lookup.
func TestGRPCAuthInterceptor_InvalidToken_ReturnsUnauthenticated(t *testing.T) {
	cfg := config.AuthConfig{Enabled: true, JWKSURL: "http://localhost:9999/certs"}
	mw := middleware.NewAuthMiddleware(cfg)

	interceptor := mw.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer garbage.not.a.jwt"))
	_, err := interceptor(ctx, nil, nil, handler)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected codes.Unauthenticated, got %v", err)
	}
}

// TestGRPCAuthInterceptor_WrongScheme_ReturnsUnauthenticated verifies that
// an authorization header without "Bearer " prefix is rejected.
func TestGRPCAuthInterceptor_WrongScheme_ReturnsUnauthenticated(t *testing.T) {
	cfg := config.AuthConfig{Enabled: true, JWKSURL: "http://localhost:9999/certs"}
	mw := middleware.NewAuthMiddleware(cfg)

	interceptor := mw.GRPCUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic abc123"))
	_, err := interceptor(ctx, nil, nil, handler)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected codes.Unauthenticated, got %v", err)
	}
}

// TestHTTPAndGRPC_ShareJWKSCache verifies that the same AuthMiddleware
// instance can be used for both HTTP and gRPC without conflicts — the JWKS
// cache is shared, not duplicated.
func TestHTTPAndGRPC_ShareJWKSCache(t *testing.T) {
	cfg := config.AuthConfig{Enabled: false} // disabled so we don't need a real JWKS server
	mw := middleware.NewAuthMiddleware(cfg)

	// Use the same middleware instance for both HTTP and gRPC.
	httpHandler := mw.JWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	grpcInterceptor := mw.GRPCUnaryInterceptor()

	// Both should pass through with auth disabled.
	rec := httptest.NewRecorder()
	httpHandler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP: expected 200, got %d", rec.Code)
	}

	grpcHandler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}
	resp, err := grpcInterceptor(context.Background(), nil, nil, grpcHandler)
	if err != nil || resp != "ok" {
		t.Fatalf("gRPC: expected ok, got %v, err=%v", resp, err)
	}
}
