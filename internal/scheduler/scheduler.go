// Package scheduler manages periodic background jobs for the Tetra Engine.
// It uses time.Ticker within goroutines managed by fx.Lifecycle for
// proper startup and graceful shutdown.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/observability"
	"github.com/anteraja/tetra-engine/internal/repository"
	"github.com/anteraja/tetra-engine/internal/service"
)

// Scheduler runs periodic background jobs.
type Scheduler struct {
	svc       *service.RecommendationService
	lockRepo  *repository.SchedulerLockRepository
	cfg       *config.Config
	metrics   *observability.Metrics
	logger    *slog.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	isRunning bool
}

const minSchedulerInterval = 5 * time.Second

// NewScheduler creates a new Scheduler and registers lifecycle hooks.
func NewScheduler(
	lc fx.Lifecycle,
	svc *service.RecommendationService,
	lockRepo *repository.SchedulerLockRepository,
	cfg *config.Config,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *Scheduler {
	s := &Scheduler{
		svc:      svc,
		lockRepo: lockRepo,
		cfg:      cfg,
		metrics:  metrics,
		logger:   logger.With(slog.String("component", "scheduler")),
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

// Start launches all scheduler goroutines after an initial sequential execution.
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
		slog.String("instance_id", s.lockRepo.InstanceID()),
		slog.Duration("order_sync", s.cfg.Scheduler.OrderSyncInterval),
		slog.Duration("carton_sync", s.cfg.Scheduler.CartonSyncInterval),
		slog.Duration("recommendation", s.cfg.Scheduler.RecommendationInterval),
		slog.Duration("push", s.cfg.Scheduler.PushInterval),
		slog.Duration("job_timeout", s.cfg.Scheduler.JobTimeout),
		slog.Duration("leader_lease_duration", s.cfg.Scheduler.LeaderLeaseDuration),
	)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.logger.Info("starting initial sequential execution")

		s.runManagedJob(ctx, "carton_sync", s.svc.SyncCartons)
		s.runManagedJob(ctx, "order_retrieval", s.svc.SyncOrders)
		s.runManagedJob(ctx, "carton_recommendation", s.svc.ProcessRecommendations)
		s.runManagedJob(ctx, "carton_push", s.svc.PushRecommendations)

		s.logger.Info("initial sequential execution complete, starting periodic jobs")

		// Now start the periodic tickers
		s.startJob(ctx, "carton_sync", s.cfg.Scheduler.CartonSyncInterval, s.svc.SyncCartons)
		s.startJob(ctx, "order_retrieval", s.cfg.Scheduler.OrderSyncInterval, s.svc.SyncOrders)
		s.startJob(ctx, "carton_recommendation", s.cfg.Scheduler.RecommendationInterval, s.svc.ProcessRecommendations)
		s.startJob(ctx, "carton_push", s.cfg.Scheduler.PushInterval, s.svc.PushRecommendations)
	}()
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
func (s *Scheduler) startJob(ctx context.Context, name string, interval time.Duration, fn func(ctx context.Context) error) {
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

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("scheduler job stopped", slog.String("job", name))
				return
			case <-ticker.C:
				s.runManagedJob(ctx, name, fn)
			}
		}
	}()
}

func (s *Scheduler) runManagedJob(ctx context.Context, name string, fn func(ctx context.Context) error) {
	s.logger.Info("running scheduled job", slog.String("job", name))

	leaderCtx, leaderCancel := context.WithTimeout(ctx, 2*time.Second)
	isLeader, lockErr := s.lockRepo.TryAcquireOrRenew(leaderCtx, s.cfg.Scheduler.LeaderLeaseDuration)
	leaderCancel()
	if lockErr != nil {
		s.metrics.IncJobFailure(name)
		s.logger.Error("leader lease check failed", slog.String("job", name), slog.Any("error", lockErr))
		return
	}

	if !isLeader {
		s.logger.Debug("skipping scheduled job on standby instance", slog.String("job", name))
		return
	}

	start := time.Now()
	jobCtx, cancel := context.WithTimeout(ctx, s.cfg.Scheduler.JobTimeout)
	err := fn(jobCtx)
	cancel()

	s.metrics.ObserveJobDuration(name, time.Since(start))
	if err != nil {
		s.metrics.IncJobFailure(name)
		s.logger.Error("scheduler job failed", slog.String("job", name), slog.Any("error", err))
	}
}

// TriggerJob manually triggers a scheduler job by name.
func (s *Scheduler) TriggerJob(ctx context.Context, jobName string) error {
	s.logger.Info("manually triggering job", slog.String("job", jobName))

	switch jobName {
	case "order_retrieval":
		s.runManagedJob(ctx, jobName, s.svc.SyncOrders)
	case "carton_sync":
		s.runManagedJob(ctx, jobName, s.svc.SyncCartons)
	case "carton_recommendation":
		s.runManagedJob(ctx, jobName, s.svc.ProcessRecommendations)
	case "carton_push":
		s.runManagedJob(ctx, jobName, s.svc.PushRecommendations)
	default:
		s.logger.Warn("unknown job name", slog.String("job", jobName))
		return fmt.Errorf("unknown job name: %s", jobName)
	}

	return nil
}

// UpdateIntervals gracefully stops the scheduler, updates the intervals, and restarts it if it was running.
func (s *Scheduler) UpdateIntervals(order, carton, rec, push time.Duration) error {
	if err := validateIntervals(order, carton, rec, push); err != nil {
		return err
	}

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

	return nil
}

// GetIntervals returns the current intervals.
func (s *Scheduler) GetIntervals() (time.Duration, time.Duration, time.Duration, time.Duration) {
	return s.cfg.Scheduler.OrderSyncInterval,
		s.cfg.Scheduler.CartonSyncInterval,
		s.cfg.Scheduler.RecommendationInterval,
		s.cfg.Scheduler.PushInterval
}

func validateIntervals(order, carton, rec, push time.Duration) error {
	intervals := map[string]time.Duration{
		"order_sync_interval":     order,
		"carton_sync_interval":    carton,
		"recommendation_interval": rec,
		"push_interval":           push,
	}

	for name, d := range intervals {
		if d < minSchedulerInterval {
			return fmt.Errorf("%s must be >= %s", name, minSchedulerInterval)
		}
	}

	return nil
}
