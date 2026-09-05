package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Scraper  ScraperConfig
}

type ServerConfig struct {
	GRPCPort    string
	GatewayPort string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	// Schema is the PostgreSQL schema owned by this service (default: lottery).
	Schema string
}

// AuthConfig configures Keycloak-based JWT validation (replaces the old
// HMAC shared-secret approach). The service validates tokens itself as
// defense-in-depth, even though Traefik also validates at the edge.
type AuthConfig struct {
	// Enabled toggles per-service JWT validation. Keep true in production.
	Enabled bool
	// JWKSURL is the Keycloak realm certs endpoint.
	JWKSURL string
	// Issuer is the expected `iss` claim (the public realm URL).
	Issuer string
	// Audience is the expected `aud` claim.
	Audience string
}

type ScraperConfig struct {
	// Cron schedule for refreshing lottery_results from pais.co.il.
	// Empty = no scheduled refresh (seed-only). Default: daily at 03:00.
	Cron string
	// SeedOnBoot seeds from the scraper on first boot if the table is empty.
	SeedOnBoot bool
}

func loadEnvFile() {
	// Try to load .env from current directory
	if err := godotenv.Load(); err != nil {
		// If not found, try to load from the module root (parent directories)
		dir, _ := os.Getwd()
		for {
			envPath := filepath.Join(dir, ".env")
			if _, err := os.Stat(envPath); err == nil {
				if loadErr := godotenv.Load(envPath); loadErr == nil {
					slog.Info("loaded .env file", "dir", dir)
					break
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				// Reached root, file not found
				slog.Info("no .env file found, using environment variables or defaults")
				break
			}
			dir = parent
		}
	}
}

func Load() (*Config, error) {
	loadEnvFile()

	cfg := &Config{
		Server: ServerConfig{
			GRPCPort:    GetEnv("GRPC_PORT", "9090"),
			GatewayPort: GetEnv("GATEWAY_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     GetEnv("DB_HOST", "localhost"),
			Port:     GetEnv("DB_PORT", "5432"),
			User:     GetEnv("DB_USER", "postgres"),
			Password: GetEnv("DB_PASSWORD", "postgres"),
			DBName:   GetEnv("DB_NAME", "statistiloto"),
			SSLMode:  GetEnv("DB_SSLMODE", "disable"),
			Schema:   GetEnv("DB_SCHEMA", "lottery"),
		},
		Auth: AuthConfig{
			Enabled:  GetEnvAsBool("AUTH_ENABLED", true),
			JWKSURL:  GetEnv("KEYCLOAK_JWKS_URL", "http://auth:8080/realms/statistiloto/protocol/openid-connect/certs"),
			Issuer:   GetEnv("KEYCLOAK_ISSUER", ""),
			Audience: GetEnv("KEYCLOAK_AUDIENCE", "statistiloto-ui"),
		},
		Scraper: ScraperConfig{
			Cron:       GetEnv("LOTTERY_SCRAPER_CRON", "0 3 * * *"),
			SeedOnBoot: GetEnvAsBool("LOTTERY_SEED_ON_BOOT", true),
		},
	}
	slog.Info("loaded config",
		"grpc_port", cfg.Server.GRPCPort,
		"gateway_port", cfg.Server.GatewayPort,
		"db_host", cfg.Database.Host,
		"db_schema", cfg.Database.Schema,
		"auth_enabled", cfg.Auth.Enabled,
		"scraper_cron", cfg.Scraper.Cron,
		"scraper_seed", cfg.Scraper.SeedOnBoot)

	return cfg, nil
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
