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

// Migrate runs the schema migrations and ensures data types are correct.
func Migrate(db *sqlx.DB, logger *slog.Logger) error {
	migrationPath := "migrations/000001_create_tables.up.sql"
	schema, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("reading migration file %s: %w", migrationPath, err)
	}

	// 1. Run basic schema creation
	_, err = db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}

	// 2. Schema Reconciliation: Fix columns that might have been created as NUMERIC in previous versions
	// This ensures everyone has the optimized INT schema regardless of when they first ran the app.
	reconcileQuery := `
		DO $$ BEGIN
			-- Ensure push_outbox_status enum exists
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'push_outbox_status') THEN
				CREATE TYPE push_outbox_status AS ENUM ('PENDING', 'RETRY', 'DELIVERED');
			END IF;

			-- Fix cartons
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'cartons' AND column_name = 'length' AND data_type = 'numeric') THEN
				ALTER TABLE cartons ALTER COLUMN length TYPE INT USING length::INT,
				                  ALTER COLUMN width TYPE INT USING width::INT,
				                  ALTER COLUMN height TYPE INT USING height::INT,
				                  ALTER COLUMN max_weight TYPE INT USING max_weight::INT;
			END IF;
			-- Fix order_items
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'order_items' AND column_name = 'length' AND data_type = 'numeric') THEN
				ALTER TABLE order_items ALTER COLUMN length TYPE INT USING length::INT,
				                      ALTER COLUMN width TYPE INT USING width::INT,
				                      ALTER COLUMN height TYPE INT USING height::INT,
				                      ALTER COLUMN weight TYPE INT USING weight::INT;
			END IF;
			-- Remove legacy column if exists
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'order_items' AND column_name = 'sku_name') THEN
				ALTER TABLE order_items DROP COLUMN sku_name;
			END IF;
			-- Add reason column to orders if missing
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'orders' AND column_name = 'reason') THEN
				ALTER TABLE orders ADD COLUMN reason TEXT;
			END IF;
			-- Add flux_id to orders if missing
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'orders' AND column_name = 'flux_id') THEN
				ALTER TABLE orders ADD COLUMN flux_id INT NOT NULL DEFAULT 0;
			END IF;
			-- Add flux_id to cartons if missing
			IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'cartons' AND column_name = 'flux_id') THEN
				ALTER TABLE cartons ADD COLUMN flux_id INT NOT NULL DEFAULT 0;
			END IF;

			-- Ensure push_outbox table exists
			CREATE TABLE IF NOT EXISTS push_outbox (
				id              BIGSERIAL           PRIMARY KEY,
				order_id        BIGINT              NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
				flux_order_id   INT                 NOT NULL,
				flux_carton_id  VARCHAR(50),
				created_by      VARCHAR(255)        NOT NULL,
				status          push_outbox_status  NOT NULL DEFAULT 'PENDING',
				attempt_count   INT                 NOT NULL DEFAULT 0,
				last_error      TEXT,
				next_attempt_at TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
				delivered_at    TIMESTAMPTZ,
				created_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
				updated_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW()
			);

			CREATE INDEX IF NOT EXISTS idx_push_outbox_status_next_attempt_at ON push_outbox (status, next_attempt_at);

			-- Ensure scheduler leader lease table exists
			CREATE TABLE IF NOT EXISTS scheduler_leader (
				id          SMALLINT     PRIMARY KEY CHECK (id = 1),
				leader_id   VARCHAR(255) NOT NULL,
				lease_until TIMESTAMPTZ  NOT NULL,
				updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
			);
		END $$;`

	_, err = db.Exec(reconcileQuery)
	if err != nil {
		logger.Warn("schema reconciliation failed (non-critical)", slog.Any("error", err))
	}

	logger.Info("database migrations and reconciliation applied successfully")
	return nil
}
