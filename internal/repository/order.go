// Package repository provides data access layer implementations
// for all Tetra Engine entities using sqlx and PostgreSQL.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/oops"

	"github.com/anteraja/tetra-engine/internal/domain"
)

// OrderRepository handles order persistence operations.
type OrderRepository struct {
	db *sqlx.DB
}

// NewOrderRepository creates a new OrderRepository.
func NewOrderRepository(db *sqlx.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// UpsertOrder inserts or updates an order by its unique code.
// Returns the local database ID of the upserted order.
func (r *OrderRepository) UpsertOrder(ctx context.Context, order *domain.Order) (int64, error) {
	query := `
		INSERT INTO orders (flux_id, code, warehouse_id, status, flux_created_at, synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (code) DO UPDATE SET
			flux_id = EXCLUDED.flux_id,
			warehouse_id = EXCLUDED.warehouse_id,
			synced_at = EXCLUDED.synced_at,
			updated_at = NOW()
		RETURNING id`

	var id int64
	err := r.db.QueryRowContext(ctx, query,
		order.FluxID,
		order.Code,
		order.WarehouseID,
		order.Status,
		order.FluxCreatedAt,
		order.SyncedAt,
	).Scan(&id)
	if err != nil {
		return 0, oops.
			In("order-repository").
			With("order_code", order.Code).
			Wrapf(err, "upserting order")
	}

	return id, nil
}

// InsertOrderItems inserts order items for a given order, handling product normalization.
// It first deletes existing items to ensure idempotency.
func (r *OrderRepository) InsertOrderItems(ctx context.Context, orderID int64, items []domain.OrderItem, products []domain.Product) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return oops.In("order-repository").Wrapf(err, "beginning transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// Upsert normalized products
	for _, p := range products {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO products (sku, name) VALUES ($1, $2)
			 ON CONFLICT (sku) DO UPDATE SET name = EXCLUDED.name`,
			p.SKU, p.Name,
		)
		if err != nil {
			return oops.In("order-repository").With("sku", p.SKU).Wrapf(err, "upserting product")
		}
	}

	// Delete existing items for idempotent re-sync
	_, err = tx.ExecContext(ctx, "DELETE FROM order_items WHERE order_id = $1", orderID)
	if err != nil {
		return oops.In("order-repository").With("order_id", orderID).Wrapf(err, "deleting existing items")
	}

	// Insert new items
	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO order_items (order_id, sku, qty, length, width, height, weight, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			orderID, item.SKU, item.Qty,
			item.Length, item.Width, item.Height, item.Weight,
		)
		if err != nil {
			return oops.
				In("order-repository").
				With("order_id", orderID).
				With("sku", item.SKU).
				Wrapf(err, "inserting order item")
		}
	}

	if err := tx.Commit(); err != nil {
		return oops.In("order-repository").Wrapf(err, "committing transaction")
	}

	return nil
}

// GetOrdersByStatus retrieves all orders with the given status.
func (r *OrderRepository) GetOrdersByStatus(ctx context.Context, status string) ([]domain.Order, error) {
	var orders []domain.Order
	err := r.db.SelectContext(ctx, &orders,
		"SELECT * FROM orders WHERE status = $1 ORDER BY id", status)
	if err != nil {
		return nil, oops.
			In("order-repository").
			With("status", status).
			Wrapf(err, "querying orders by status")
	}
	return orders, nil
}

// GetOrdersByStatuses retrieves all orders with any of the given statuses.
func (r *OrderRepository) GetOrdersByStatuses(ctx context.Context, statuses []string) ([]domain.Order, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In("SELECT * FROM orders WHERE status IN (?) ORDER BY id", statuses)
	if err != nil {
		return nil, oops.In("order-repository").Wrapf(err, "building IN query")
	}
	query = r.db.Rebind(query)

	var orders []domain.Order
	err = r.db.SelectContext(ctx, &orders, query, args...)
	if err != nil {
		return nil, oops.In("order-repository").Wrapf(err, "querying orders by statuses")
	}
	return orders, nil
}

// GetOrderItemsByOrderID retrieves all items for a given order.
func (r *OrderRepository) GetOrderItemsByOrderID(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	var items []domain.OrderItem
	err := r.db.SelectContext(ctx, &items,
		"SELECT * FROM order_items WHERE order_id = $1", orderID)
	if err != nil {
		return nil, oops.
			In("order-repository").
			With("order_id", orderID).
			Wrapf(err, "querying order items")
	}
	return items, nil
}

// UpdateOrderStatus updates the status, related timestamp, and reasoning of an order.
func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID int64, status string, cartonID *int64, reason *string) error {
	var query string
	var args []interface{}

	now := time.Now()

	switch status {
	case domain.OrderStatusPending:
		query = "UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3"
		args = []interface{}{status, now, orderID}
	case domain.OrderStatusRecommended:
		query = "UPDATE orders SET status = $1, carton_id = $2, reason = $3, recommended_at = $4, updated_at = $4 WHERE id = $5"
		args = []interface{}{status, cartonID, reason, now, orderID}
	case domain.OrderStatusPushed:
		query = "UPDATE orders SET status = $1, pushed_at = $2, updated_at = $2 WHERE id = $3"
		args = []interface{}{status, now, orderID}
	case domain.OrderStatusError:
		query = "UPDATE orders SET status = $1, reason = $2, updated_at = $3 WHERE id = $4"
		args = []interface{}{status, reason, now, orderID}
	default:
		query = "UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3"
		args = []interface{}{status, now, orderID}
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return oops.
			In("order-repository").
			With("order_id", orderID).
			With("status", status).
			Wrapf(err, "updating order status")
	}

	return nil
}

// GetOrderByCode retrieves an order by its code, returns nil if not found.
func (r *OrderRepository) GetOrderByCode(ctx context.Context, code string) (*domain.Order, error) {
	var order domain.Order
	err := r.db.GetContext(ctx, &order, "SELECT * FROM orders WHERE code = $1", code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, oops.
			In("order-repository").
			With("code", code).
			Wrapf(err, "querying order by code")
	}
	return &order, nil
}

// MarkSyncedAsPending transitions all SYNCED orders to PENDING status.
func (r *OrderRepository) MarkSyncedAsPending(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		"UPDATE orders SET status = $1, updated_at = NOW() WHERE status = $2",
		domain.OrderStatusPending, domain.OrderStatusSynced)
	if err != nil {
		return 0, oops.In("order-repository").Wrapf(err, "marking synced as pending")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, oops.In("order-repository").Wrapf(err, "getting rows affected")
	}
	return count, nil
}
