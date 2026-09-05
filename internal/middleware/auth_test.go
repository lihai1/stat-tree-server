package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lihai1/stat-tree-server/internal/config"
	"github.com/lihai1/stat-tree-server/internal/middleware"
)

// newMiddleware builds an AuthMiddleware from the given AuthConfig and wraps
// a trivial "ok" handler so we can assert pass-through vs rejection.
func newMiddleware(t *testing.T, cfg config.AuthConfig) (*middleware.AuthMiddleware, http.Handler) {
	t.Helper()
	mw := middleware.NewAuthMiddleware(cfg)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mw, mw.JWT(next)
}

// TestAuthMiddleware_AuthDisabled_PassesThrough verifies that when auth is
// disabled the middleware is a transparent pass-through (no token required).
func TestAuthMiddleware_AuthDisabled_PassesThrough(t *testing.T) {
	cfg := config.AuthConfig{Enabled: false}
	_, handler := newMiddleware(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Del("Authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", rec.Body.String())
	}
}

// TestAuthMiddleware_MissingToken_Returns401 verifies that with auth enabled
// a request lacking the Authorization header is rejected with 401.
func TestAuthMiddleware_MissingToken_Returns401(t *testing.T) {
	cfg := config.AuthConfig{
		Enabled:  true,
		JWKSURL:  "http://localhost:8080/realms/test/protocol/openid-connect/certs",
		Issuer:   "http://localhost/realms/test",
		Audience: "test-client",
	}
	_, handler := newMiddleware(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Del("Authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d (body=%q)", rec.Code, rec.Body.String())
	}
}

// TestAuthMiddleware_InvalidToken_Returns401 verifies that a malformed
// Bearer token is rejected with 401 before any JWKS lookup is attempted.
func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	cfg := config.AuthConfig{
		Enabled:  true,
		JWKSURL:  "http://localhost:8080/realms/test/protocol/openid-connect/certs",
		Issuer:   "http://localhost/realms/test",
		Audience: "test-client",
	}
	_, handler := newMiddleware(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer garbage.not.a.jwt")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d (body=%q)", rec.Code, rec.Body.String())
	}
}
