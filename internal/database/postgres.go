package database

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lihai1/stat-tree-server/internal/config"
)

type Database struct {
	Pool *pgxpool.Pool
}

func NewDatabase(cfg *config.DatabaseConfig) (*Database, error) {
	connString := buildConnectionString(cfg)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection config: %w", err)
	}

	// Set the search_path to the service's own schema so all unqualified
	// table references (lottery_results) resolve correctly in the shared
	// PostgreSQL instance.
	if cfg.Schema != "" {
		poolConfig.ConnConfig.RuntimeParams["search_path"] = cfg.Schema
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Printf("Successfully connected to database (schema: %s)", cfg.Schema)
	return &Database{Pool: pool}, nil
}

func buildConnectionString(cfg *config.DatabaseConfig) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)
}

func (db *Database) Close() {
	db.Pool.Close()
}
