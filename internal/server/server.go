// Package server provides the HTTP server for health checks and manual triggers.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/scheduler"
)

// Server is the HTTP server for the Tetra Engine.
type Server struct {
	srv       *http.Server
	db        *sqlx.DB
	scheduler *scheduler.Scheduler
	logger    *slog.Logger
}

// NewServer creates and registers the HTTP server with fx.Lifecycle.
func NewServer(
	lc fx.Lifecycle,
	db *sqlx.DB,
	sched *scheduler.Scheduler,
	cfg *config.Config,
	logger *slog.Logger,
) *Server {
	mux := http.NewServeMux()
	s := &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
			Handler: mux,
		},
		db:        db,
		scheduler: sched,
		logger:    logger.With(slog.String("component", "http-server")),
	}

	// Register routes
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/internal/trigger/{job_name}", corsMiddleware(s.handleTrigger))

	// Dashboard API routes (CORS-enabled for local Vue frontend)
	mux.HandleFunc("GET /api/dashboard/stats", corsMiddleware(s.handleDashboardStats))
	mux.HandleFunc("GET /api/dashboard/orders", corsMiddleware(s.handleDashboardOrders))
	mux.HandleFunc("GET /api/dashboard/orders/{id}", corsMiddleware(s.handleDashboardOrderDetail))
	mux.HandleFunc("GET /api/dashboard/cartons", corsMiddleware(s.handleDashboardCartons))
	mux.HandleFunc("GET /api/dashboard/logs", corsMiddleware(s.handleDashboardLogs))
	mux.HandleFunc("GET /api/dashboard/engine/status", corsMiddleware(s.handleEngineStatus))
	mux.HandleFunc("POST /api/dashboard/engine/start", corsMiddleware(s.handleEngineStart))
	mux.HandleFunc("POST /api/dashboard/engine/stop", corsMiddleware(s.handleEngineStop))
	mux.HandleFunc("POST /api/dashboard/engine/reset", corsMiddleware(s.handleEngineReset))
	mux.HandleFunc("GET /api/dashboard/settings", corsMiddleware(s.handleDashboardSettingsGet))
	mux.HandleFunc("POST /api/dashboard/settings", corsMiddleware(s.handleDashboardSettingsUpdate))
	mux.HandleFunc("OPTIONS /api/dashboard/{path...}", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {}))
	mux.HandleFunc("OPTIONS /api/internal/{path...}", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {}))

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", s.srv.Addr)
			if err != nil {
				return fmt.Errorf("listening on %s: %w", s.srv.Addr, err)
			}
			s.logger.Info("HTTP server started", slog.String("addr", s.srv.Addr))
			go s.srv.Serve(ln) //nolint:errcheck
			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.logger.Info("shutting down HTTP server")
			return s.srv.Shutdown(ctx)
		},
	})

	return s
}

// handleHealth checks database connectivity and returns server status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := s.db.PingContext(ctx); err != nil {
		s.logger.Error("health check failed", slog.Any("error", err))
		http.Error(w, `{"status":"unhealthy","error":"database connection failed"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"healthy"}`)
}

// handleTrigger manually triggers a scheduler job.
func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	jobName := r.PathValue("job_name")
	if jobName == "" {
		http.Error(w, `{"error":"job_name is required"}`, http.StatusBadRequest)
		return
	}

	validJobs := map[string]bool{
		"order_retrieval":      true,
		"carton_sync":          true,
		"carton_recommendation": true,
		"carton_push":          true,
	}

	if !validJobs[jobName] {
		http.Error(w, fmt.Sprintf(`{"error":"unknown job: %s"}`, jobName), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := s.scheduler.TriggerJob(ctx, jobName); err != nil {
		s.logger.Error("manual trigger failed",
			slog.String("job", jobName),
			slog.Any("error", err),
		)
		http.Error(w, fmt.Sprintf(`{"error":"job failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"message":"job %s triggered successfully"}`, jobName)
}
