package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/lihai1/stat-tree-server/internal/config"
)

// AuthMiddleware validates Keycloak-issued JWT (RS256) tokens against the
// realm JWKS endpoint. This replaces the old HMAC shared-secret approach —
// the service is now stateless and trusts Keycloak as the sole identity
// issuer. Traefik also validates at the edge; this is defense-in-depth.
type AuthMiddleware struct {
	cfg    config.AuthConfig
	jwks   *jwksCache
	client *http.Client
}

// jwksCache fetches and caches Keycloak JWKS keys, refreshing periodically.
type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
	jwksURL   string
	client    *http.Client
}

type jwksKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

func newJWKSCache(jwksURL string, client *http.Client) *jwksCache {
	return &jwksCache{
		keys:    make(map[string]*rsa.PublicKey),
		ttl:     15 * time.Minute,
		jwksURL: jwksURL,
		client:  client,
	}
}

// get returns the RSA public key for the given kid, refreshing the cache
// if it is stale or the kid is unknown.
func (c *jwksCache) get(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if key, ok := c.keys[kid]; ok && time.Since(c.fetchedAt) < c.ttl {
		c.mu.RUnlock()
		return key, nil
	}
	c.mu.RUnlock()

	if err := c.refresh(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if key, ok := c.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("key not found for kid: %s", kid)
}

func (c *jwksCache) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if len(c.keys) > 0 && time.Since(c.fetchedAt) < c.ttl {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create JWKS request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JWKS response: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("failed to parse JWKS JSON: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	if len(keys) == 0 {
		return fmt.Errorf("no valid RSA signing keys in JWKS")
	}

	c.keys = keys
	c.fetchedAt = time.Now()
	return nil
}

// parseRSAPublicKey converts base64url-encoded JWK modulus/exponent into an
// *rsa.PublicKey.
func parseRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	n, err := base64URLDecode(nB64)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus: %w", err)
	}
	e, err := base64URLDecode(eB64)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent: %w", err)
	}

	exponent := 0
	for _, b := range e {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("invalid exponent value")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(n),
		E: exponent,
	}, nil
}

func base64URLDecode(s string) ([]byte, error) {
	// JWK uses base64url without padding. RawURLEncoding handles unpadded input.
	// If the input happens to have padding, strip it first (RawURLEncoding rejects '=').
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

func NewAuthMiddleware(cfg config.AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (am *AuthMiddleware) keyFunc() *jwksCache {
	if am.jwks == nil {
		am.jwks = newJWKSCache(am.cfg.JWKSURL, am.client)
	}
	return am.jwks
}

// JWT returns an http.Handler that validates the Bearer token before
// calling the next handler. If auth is disabled (e.g. local dev), it
// passes through.
func (am *AuthMiddleware) JWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !am.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if err := am.validateAuthHeader(authHeader); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateAuthHeader parses and validates a "Bearer <token>" string using
// the shared JWKS cache, issuer, and audience checks. Returns nil on success.
// This is the core validation logic shared by both the HTTP middleware and
// the gRPC interceptor.
func (am *AuthMiddleware) validateAuthHeader(authHeader string) error {
	if authHeader == "" {
		return fmt.Errorf("Authorization header required")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return fmt.Errorf("Invalid authorization header format")
	}

	return am.validateTokenString(tokenString)
}

// validateTokenString parses and validates a raw JWT string.
func (am *AuthMiddleware) validateTokenString(tokenString string) error {
	claims := jwt.MapClaims{}
	cache := am.keyFunc()

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, _ := token.Header["kid"].(string)
		return cache.get(kid)
	}, jwt.WithValidMethods([]string{"RS256"}))

	if err != nil || !token.Valid {
		slog.Warn("token validation failed", "error", err, "valid", token != nil && token.Valid)
		return fmt.Errorf("Invalid token")
	}

	// Verify issuer if configured.
	if am.cfg.Issuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != am.cfg.Issuer {
			return fmt.Errorf("Invalid token issuer")
		}
	}

	// Verify audience if configured.
	if am.cfg.Audience != "" {
		if !audienceContains(claims["aud"], am.cfg.Audience) {
			return fmt.Errorf("Invalid token audience")
		}
	}

	return nil
}

// GRPCUnaryInterceptor returns a gRPC unary server interceptor that validates
// the JWT from the "authorization" gRPC metadata entry. It reuses the same
// AuthMiddleware instance (and its JWKS cache) as the HTTP middleware, so
// both transport paths share a single validation engine.
func (am *AuthMiddleware) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !am.cfg.Enabled {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "Authorization metadata required")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "Authorization header required")
		}

		if err := am.validateAuthHeader(values[0]); err != nil {
			slog.Warn("gRPC auth: token validation failed", "error", err)
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		return handler(ctx, req)
	}
}

// audienceContains checks whether the expected audience is present in the
// "aud" claim. The claim can be either a single string or a slice of strings
// (per RFC 7519 §4.1.3).
func audienceContains(aud any, expected string) bool {
	switch v := aud.(type) {
	case string:
		return v == expected
	case []string:
		for _, a := range v {
			if a == expected {
				return true
			}
		}
	case []any:
		for _, a := range v {
			if fmt.Sprintf("%v", a) == expected {
				return true
			}
		}
	}
	return false
}
