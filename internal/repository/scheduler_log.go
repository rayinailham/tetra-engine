package repository

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/oops"

	"github.com/anteraja/tetra-engine/internal/domain"
)

// SchedulerLogRepository handles scheduler execution logging.
type SchedulerLogRepository struct {
	db *sqlx.DB
}

// NewSchedulerLogRepository creates a new SchedulerLogRepository.
func NewSchedulerLogRepository(db *sqlx.DB) *SchedulerLogRepository {
	return &SchedulerLogRepository{db: db}
}

// StartLog creates a new scheduler log entry with RUNNING status and returns its ID.
func (r *SchedulerLogRepository) StartLog(ctx context.Context, schedulerName string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO scheduler_logs (scheduler_name, status, started_at)
		 VALUES ($1, $2, NOW())
		 RETURNING id`,
		schedulerName, domain.SchedulerStatusRunning,
	).Scan(&id)
	if err != nil {
		return 0, oops.
			In("scheduler-log-repository").
			With("scheduler_name", schedulerName).
			Wrapf(err, "creating scheduler log")
	}
	return id, nil
}

// FinishLog updates a scheduler log entry with the final status and results.
func (r *SchedulerLogRepository) FinishLog(ctx context.Context, logID int64, status string, recordsProcessed int, errMsg *string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE scheduler_logs
		 SET status = $1, records_processed = $2, error_message = $3, finished_at = $4
		 WHERE id = $5`,
		status, recordsProcessed, errMsg, now, logID,
	)
	if err != nil {
		return oops.
			In("scheduler-log-repository").
			With("log_id", logID).
			Wrapf(err, "finishing scheduler log")
	}
	return nil
}
