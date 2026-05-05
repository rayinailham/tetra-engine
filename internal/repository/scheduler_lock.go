package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/oops"
)

// SchedulerLockRepository coordinates one active scheduler across instances.
type SchedulerLockRepository struct {
	db         *sqlx.DB
	instanceID string
}

// NewSchedulerLockRepository creates a new SchedulerLockRepository.
func NewSchedulerLockRepository(db *sqlx.DB) *SchedulerLockRepository {
	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s:%d", hostname, os.Getpid())

	return &SchedulerLockRepository{db: db, instanceID: instanceID}
}

func (r *SchedulerLockRepository) InstanceID() string {
	return r.instanceID
}

// TryAcquireOrRenew acquires leadership when expired, or renews if already leader.
func (r *SchedulerLockRepository) TryAcquireOrRenew(ctx context.Context, leaseDuration time.Duration) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO scheduler_leader (id, leader_id, lease_until, updated_at)
		VALUES (1, $1, NOW() + $2::interval, NOW())
		ON CONFLICT (id) DO UPDATE SET
			leader_id = EXCLUDED.leader_id,
			lease_until = EXCLUDED.lease_until,
			updated_at = NOW()
		WHERE scheduler_leader.lease_until < NOW()
		   OR scheduler_leader.leader_id = EXCLUDED.leader_id`,
		r.instanceID, postgresInterval(leaseDuration))
	if err != nil {
		return false, oops.In("scheduler-lock-repository").Wrapf(err, "acquiring scheduler leader lease")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, oops.In("scheduler-lock-repository").Wrapf(err, "reading leader lease rows affected")
	}

	return rows > 0, nil
}

func postgresInterval(d time.Duration) string {
	if d <= 0 {
		d = 1 * time.Second
	}

	seconds := int64(d / time.Second)
	if seconds <= 0 {
		seconds = 1
	}

	return fmt.Sprintf("%d seconds", seconds)
}
