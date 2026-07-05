package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Port        string
	Host        string
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
}

type JWTConfig struct {
	Secret string
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
					log.Printf("Loaded .env from %s", dir)
					break
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				// Reached root, file not found
				log.Printf("No .env file found, using environment variables or defaults")
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
			Port:        GetEnv("SERVER_PORT", "8080"),
			Host:        GetEnv("SERVER_HOST", "0.0.0.0"),
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
		},
		JWT: JWTConfig{
			Secret: GetEnv("JWT_SECRET", "your-secret-key"),
		},
	}
	log.Printf("Loaded config: Server{Port:%s, Host:%s, GRPC:%s, Gateway:%s}, Database{Host:%s, Port:%s, DB:%s}",
		cfg.Server.Port, cfg.Server.Host, cfg.Server.GRPCPort, cfg.Server.GatewayPort,
		cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)

	return cfg, nil
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
