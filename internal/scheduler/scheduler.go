// Package scheduler manages periodic background jobs for the Tetra Engine.
// It uses time.Ticker within goroutines managed by fx.Lifecycle for
// proper startup and graceful shutdown.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/service"
)

// Scheduler runs periodic background jobs.
type Scheduler struct {
	svc    *service.RecommendationService
	cfg    *config.Config
	logger    *slog.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	isRunning bool
}

// NewScheduler creates a new Scheduler and registers lifecycle hooks.
func NewScheduler(
	lc fx.Lifecycle,
	svc *service.RecommendationService,
	cfg *config.Config,
	logger *slog.Logger,
) *Scheduler {
	s := &Scheduler{
		svc:    svc,
		cfg:    cfg,
		logger: logger.With(slog.String("component", "scheduler")),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.Stop()
			return nil
		},
	})

	return s
}

// Start launches all scheduler goroutines.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		s.logger.Info("scheduler already running")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.isRunning = true

	s.logger.Info("starting schedulers",
		slog.Duration("order_sync", s.cfg.Scheduler.OrderSyncInterval),
		slog.Duration("carton_sync", s.cfg.Scheduler.CartonSyncInterval),
		slog.Duration("recommendation", s.cfg.Scheduler.RecommendationInterval),
		slog.Duration("push", s.cfg.Scheduler.PushInterval),
	)

	// Job 1: Order Retrieval Sync
	s.startJob(ctx, "order_retrieval", s.cfg.Scheduler.OrderSyncInterval, func(ctx context.Context) {
		if err := s.svc.SyncOrders(ctx); err != nil {
			s.logger.Error("order sync failed", slog.Any("error", err))
		}
	})

	// Job 2: Carton Master Sync
	s.startJob(ctx, "carton_sync", s.cfg.Scheduler.CartonSyncInterval, func(ctx context.Context) {
		if err := s.svc.SyncCartons(ctx); err != nil {
			s.logger.Error("carton sync failed", slog.Any("error", err))
		}
	})

	// Job 3: Carton Recommendation
	s.startJob(ctx, "carton_recommendation", s.cfg.Scheduler.RecommendationInterval, func(ctx context.Context) {
		if err := s.svc.ProcessRecommendations(ctx); err != nil {
			s.logger.Error("recommendation processing failed", slog.Any("error", err))
		}
	})

	// Job 4: Carton Push
	s.startJob(ctx, "carton_push", s.cfg.Scheduler.PushInterval, func(ctx context.Context) {
		if err := s.svc.PushRecommendations(ctx); err != nil {
			s.logger.Error("carton push failed", slog.Any("error", err))
		}
	})
}

// Stop gracefully shuts down all scheduler goroutines.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.logger.Info("stopping schedulers...")
	if s.cancel != nil {
		s.cancel()
	}
	s.isRunning = false
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("all schedulers stopped")
}

// Status returns whether the engine is currently running.
func (s *Scheduler) Status() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// startJob launches a periodic job in a goroutine with panic recovery.
func (s *Scheduler) startJob(ctx context.Context, name string, interval time.Duration, fn func(ctx context.Context)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("scheduler job panicked",
					slog.String("job", name),
					slog.Any("panic", r),
				)
			}
		}()

		// Run immediately on start
		s.logger.Info("running initial job execution", slog.String("job", name))
		fn(ctx)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("scheduler job stopped", slog.String("job", name))
				return
			case <-ticker.C:
				s.logger.Info("running scheduled job", slog.String("job", name))
				fn(ctx)
			}
		}
	}()
}

// TriggerJob manually triggers a scheduler job by name.
func (s *Scheduler) TriggerJob(ctx context.Context, jobName string) error {
	s.logger.Info("manually triggering job", slog.String("job", jobName))

	switch jobName {
	case "order_retrieval":
		return s.svc.SyncOrders(ctx)
	case "carton_sync":
		return s.svc.SyncCartons(ctx)
	case "carton_recommendation":
		return s.svc.ProcessRecommendations(ctx)
	case "carton_push":
		return s.svc.PushRecommendations(ctx)
	default:
		s.logger.Warn("unknown job name", slog.String("job", jobName))
		return nil
	}
}

// UpdateIntervals gracefully stops the scheduler, updates the intervals, and restarts it if it was running.
func (s *Scheduler) UpdateIntervals(order, carton, rec, push time.Duration) {
	s.mu.Lock()
	wasRunning := s.isRunning
	s.mu.Unlock()

	if wasRunning {
		s.Stop()
	}

	s.cfg.Scheduler.OrderSyncInterval = order
	s.cfg.Scheduler.CartonSyncInterval = carton
	s.cfg.Scheduler.RecommendationInterval = rec
	s.cfg.Scheduler.PushInterval = push

	if wasRunning {
		s.Start()
	}
}

// GetIntervals returns the current intervals.
func (s *Scheduler) GetIntervals() (time.Duration, time.Duration, time.Duration, time.Duration) {
	return s.cfg.Scheduler.OrderSyncInterval,
		s.cfg.Scheduler.CartonSyncInterval,
		s.cfg.Scheduler.RecommendationInterval,
		s.cfg.Scheduler.PushInterval
}
