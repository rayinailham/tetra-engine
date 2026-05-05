package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/oops"

	"github.com/anteraja/tetra-engine/internal/domain"
)

// OutboxRepository handles reliable push delivery state.
type OutboxRepository struct {
	db *sqlx.DB
}

// NewOutboxRepository creates a new OutboxRepository.
func NewOutboxRepository(db *sqlx.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// EnqueuePushIntent inserts one push intent for an order if missing.
func (r *OutboxRepository) EnqueuePushIntent(ctx context.Context, order domain.Order, fluxCartonID *string, createdBy string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO push_outbox (
			order_id, flux_order_id, flux_carton_id, created_by, status, attempt_count, next_attempt_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 0, NOW(), NOW(), NOW())
		ON CONFLICT (order_id) DO NOTHING`,
		order.ID, order.FluxID, fluxCartonID, createdBy, domain.PushOutboxStatusPending)
	if err != nil {
		return false, oops.In("outbox-repository").With("order_id", order.ID).Wrapf(err, "enqueuing push intent")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, oops.In("outbox-repository").With("order_id", order.ID).Wrapf(err, "reading enqueue rows affected")
	}

	return rows > 0, nil
}

// ListPendingPushes returns pending/retry outbox records that are due.
func (r *OutboxRepository) ListPendingPushes(ctx context.Context, limit int) ([]domain.PushOutbox, error) {
	var rows []domain.PushOutbox
	err := r.db.SelectContext(ctx, &rows, `
		SELECT *
		FROM push_outbox
		WHERE status IN ($1, $2) AND next_attempt_at <= NOW()
		ORDER BY id ASC
		LIMIT $3`, domain.PushOutboxStatusPending, domain.PushOutboxStatusRetry, limit)
	if err != nil {
		return nil, oops.In("outbox-repository").Wrapf(err, "listing pending pushes")
	}

	return rows, nil
}

// MarkDelivered marks outbox row as delivered and updates order status in one transaction.
func (r *OutboxRepository) MarkDelivered(ctx context.Context, outboxID int64, orderID int64, cartonID *int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return oops.In("outbox-repository").Wrapf(err, "beginning mark delivered transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		UPDATE push_outbox
		SET status = $1,
			delivered_at = NOW(),
			updated_at = NOW()
		WHERE id = $2`,
		domain.PushOutboxStatusDelivered, outboxID)
	if err != nil {
		return oops.In("outbox-repository").With("outbox_id", outboxID).Wrapf(err, "marking outbox as delivered")
	}

	if err := updateOrderStatusTx(ctx, tx, orderID, domain.OrderStatusPushed, cartonID); err != nil {
		return oops.In("outbox-repository").With("outbox_id", outboxID).Wrapf(err, "marking order as pushed")
	}

	if err := tx.Commit(); err != nil {
		return oops.In("outbox-repository").With("outbox_id", outboxID).Wrapf(err, "committing mark delivered transaction")
	}

	return nil
}

// MarkRetry updates retry state with backoff and error details.
func (r *OutboxRepository) MarkRetry(ctx context.Context, outboxID int64, errMsg string, backoff time.Duration) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE push_outbox
		SET status = $1,
			attempt_count = attempt_count + 1,
			last_error = $2,
			next_attempt_at = NOW() + $3::interval,
			updated_at = NOW()
		WHERE id = $4`,
		domain.PushOutboxStatusRetry, errMsg, formatPostgresInterval(backoff), outboxID)
	if err != nil {
		return oops.In("outbox-repository").With("outbox_id", outboxID).Wrapf(err, "marking outbox retry")
	}

	return nil
}

func updateOrderStatusTx(ctx context.Context, tx *sqlx.Tx, orderID int64, status string, cartonID *int64) error {
	now := time.Now()
	_, err := tx.ExecContext(ctx,
		"UPDATE orders SET status = $1, carton_id = COALESCE($2, carton_id), pushed_at = $3, updated_at = $3 WHERE id = $4",
		status, cartonID, now, orderID)
	if err != nil {
		return oops.In("outbox-repository").With("order_id", orderID).With("status", status).Wrapf(err, "updating order status in tx")
	}

	return nil
}

func formatPostgresInterval(d time.Duration) string {
	seconds := int64(d / time.Second)
	if seconds <= 0 {
		seconds = 1
	}

	return fmt.Sprintf("%d seconds", seconds)
}
