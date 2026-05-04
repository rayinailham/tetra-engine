// Package main is the entry point for the Tetra Recommendation Engine.
// It wires all dependencies using uber-go/fx and starts the application
// with proper lifecycle management and graceful shutdown.
package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/anteraja/tetra-engine/internal/client"
	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/database"
	"github.com/anteraja/tetra-engine/internal/repository"
	"github.com/anteraja/tetra-engine/internal/scheduler"
	"github.com/anteraja/tetra-engine/internal/server"
	"github.com/anteraja/tetra-engine/internal/service"
)

func main() {
	fx.New(
		// Configuration
		fx.Provide(provideConfig),
		fx.Provide(provideLogger),

		// Database
		fx.Provide(provideDatabase),

		// Repositories
		fx.Provide(repository.NewOrderRepository),
		fx.Provide(repository.NewCartonRepository),
		fx.Provide(repository.NewSchedulerLogRepository),

		// External client
		fx.Provide(client.NewFluxClient),

		// Service
		fx.Provide(service.NewRecommendationService),

		// Scheduler (registers lifecycle hooks)
		fx.Provide(scheduler.NewScheduler),

		// HTTP server (registers lifecycle hooks)
		fx.Provide(server.NewServer),

		// Invoke to ensure Scheduler and Server are instantiated
		fx.Invoke(func(*scheduler.Scheduler) {}),
		fx.Invoke(func(*server.Server) {}),
	).Run()
}

func provideConfig() (*config.Config, error) {
	return config.Load()
}

func provideLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Log.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

func provideDatabase(cfg *config.Config, logger *slog.Logger) (*sqlx.DB, error) {
	return database.New(cfg, logger)
}
