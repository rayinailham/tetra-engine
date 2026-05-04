// Package database provides the database connection setup and lifecycle
// management for the Tetra Engine using sqlx and PostgreSQL.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/anteraja/tetra-engine/internal/config"
)

// New creates a new sqlx database connection with proper pool settings.
func New(cfg *config.Config, logger *slog.Logger) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}

	// Connection pool configuration per golang-database skill
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	logger.Info("database connection established",
		slog.String("host", cfg.Database.Host),
		slog.Int("port", cfg.Database.Port),
		slog.String("database", cfg.Database.Name),
	)

	// Run migrations automatically on startup
	if err := Migrate(db, logger); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// Migrate runs the schema migrations located in the migrations directory.
func Migrate(db *sqlx.DB, logger *slog.Logger) error {
	migrationPath := "migrations/000001_create_tables.up.sql"
	schema, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("reading migration file %s: %w", migrationPath, err)
	}

	_, err = db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}

	logger.Info("database migrations applied successfully")
	return nil
}
