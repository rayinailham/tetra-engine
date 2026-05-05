// Package main is the entry point for the Tetra Recommendation Engine.
// It wires all dependencies using uber-go/fx and starts the application
// with proper lifecycle management and graceful shutdown.
package main

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lmittmann/tint"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/anteraja/tetra-engine/internal/client"
	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/database"
	"github.com/anteraja/tetra-engine/internal/observability"
	"github.com/anteraja/tetra-engine/internal/repository"
	"github.com/anteraja/tetra-engine/internal/scheduler"
	"github.com/anteraja/tetra-engine/internal/server"
	"github.com/anteraja/tetra-engine/internal/service"
)

func main() {
	fx.New(
		// Redirect Fx events to slog
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: log}
		}),

		// Configuration
		fx.Provide(provideConfig),
		fx.Provide(provideLogger),

		// Database
		fx.Provide(provideDatabase),
		fx.Provide(observability.NewMetrics),

		// Repositories
		fx.Provide(repository.NewOrderRepository),
		fx.Provide(repository.NewCartonRepository),
		fx.Provide(repository.NewSchedulerLogRepository),
		fx.Provide(repository.NewOutboxRepository),
		fx.Provide(repository.NewSchedulerLockRepository),

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

	var handler slog.Handler
	switch strings.ToLower(cfg.Log.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: level == slog.LevelDebug,
		})
	default:
		// Human-friendly text logs using tint
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
			AddSource:  level == slog.LevelDebug,
			NoColor:    false, // Enable color for human readability
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// Handle multiline strings (like stacktraces) more cleanly in the terminal
				if a.Value.Kind() == slog.KindString {
					val := a.Value.String()
					if strings.Contains(val, "\n") {
						// For the terminal, we want to see the newlines
						// tint preserves newlines if we return the string as is
						return a
					}
				}

				// Simplify Fx event names for human readability
				if a.Key == "event" {
					if val, ok := a.Value.Any().(string); ok {
						if strings.HasPrefix(val, "*fxevent.") {
							return slog.String(a.Key, strings.TrimPrefix(val, "*fxevent."))
						}
					}
				}

				return a
			},
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

func provideDatabase(cfg *config.Config, logger *slog.Logger) (*sqlx.DB, error) {
	return database.New(cfg, logger)
}
