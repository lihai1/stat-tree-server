package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

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
	// Add padding if needed.
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
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
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

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
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Verify issuer if configured.
		if am.cfg.Issuer != "" {
			iss, _ := claims["iss"].(string)
			if iss != am.cfg.Issuer {
				http.Error(w, "Invalid token issuer", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
