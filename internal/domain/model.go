// Package domain defines the core business models for the Tetra Engine.
// These models represent the data structures used across all layers —
// API responses, database entities, and business logic.
package domain

import (
	"time"
)

// Order status constants define the lifecycle of an order.
const (
	OrderStatusSynced      = "SYNCED"
	OrderStatusPending     = "PENDING"
	OrderStatusRecommended = "RECOMMENDED"
	OrderStatusPushed      = "PUSHED"
	OrderStatusError       = "NO RECOMMENDATION"
)

// Scheduler log status constants.
const (
	SchedulerStatusRunning = "RUNNING"
	SchedulerStatusSuccess = "SUCCESS"
	SchedulerStatusFailed  = "FAILED"
)

// Order represents an order record in the Tetra database.
type Order struct {
	ID            int64      `db:"id"`
	Code          string     `db:"code"`
	WarehouseID   string     `db:"warehouse_id"`
	Status        string     `db:"status"`
	CartonID      *int64     `db:"carton_id"`
	FluxCreatedAt *time.Time `db:"flux_created_at"`
	SyncedAt      *time.Time `db:"synced_at"`
	RecommendedAt *time.Time `db:"recommended_at"`
	PushedAt      *time.Time `db:"pushed_at"`
	Reason        *string    `db:"reason"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

// OrderItem represents an item within an order.
type OrderItem struct {
	ID        int64     `db:"id"`
	OrderID   int64     `db:"order_id"`
	SKU       string    `db:"sku"`
	Qty       int       `db:"qty"`
	Length    int       `db:"length"` // in millimeters
	Width     int       `db:"width"`  // in millimeters
	Height    int       `db:"height"` // in millimeters
	Weight    int       `db:"weight"` // in grams
	CreatedAt time.Time `db:"created_at"`
}

// Product represents a normalized product sku.
type Product struct {
	SKU  string  `db:"sku"`
	Name *string `db:"name"`
}

// Carton represents a carton master data record.
type Carton struct {
	ID        int64     `db:"id"`
	Code      string    `db:"code"`
	Length    int       `db:"length"`     // in millimeters
	Width     int       `db:"width"`      // in millimeters
	Height    int       `db:"height"`     // in millimeters
	MaxWeight int       `db:"max_weight"` // in grams
	IsActive  bool      `db:"is_active"`
	SyncedAt  *time.Time `db:"synced_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

// Volume returns the internal volume of the carton in cubic millimeters.
func (c Carton) Volume() int {
	return c.Length * c.Width * c.Height
}

// SchedulerLog records a single scheduler execution.
type SchedulerLog struct {
	ID               int64      `db:"id"`
	SchedulerName    string     `db:"scheduler_name"`
	Status           string     `db:"status"`
	RecordsProcessed *int       `db:"records_processed"`
	ErrorMessage     *string    `db:"error_message"`
	StartedAt        time.Time  `db:"started_at"`
	FinishedAt       *time.Time `db:"finished_at"`
}

// FluxOrder represents an order as returned by the Flux WMS API (GET /orders).
type FluxOrder struct {
	ID          int     `json:"id"`
	Code        string  `json:"code"`
	WarehouseID string  `json:"warehouse_id"`
	CartonID    *string `json:"carton_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// FluxOrderDetail represents the response from GET /orders/{id}.
type FluxOrderDetail struct {
	Order   FluxOrder       `json:"order"`
	Details []FluxOrderItem `json:"details"`
}

// FluxOrderItem represents an item within a Flux order detail response.
type FluxOrderItem struct {
	ID        int    `json:"id"`
	OrderID   int    `json:"order_id"`
	SKU       string `json:"sku"`
	SKUName   string `json:"sku_name"`
	Qty       int    `json:"qty"`
	Length    string `json:"length"`
	Width     string `json:"width"`
	Height    string `json:"height"`
	Weight    string `json:"weight"`
	CreatedAt string `json:"created_at"`
}

// FluxCarton represents a carton as returned by the Flux WMS API (GET /cartons).
type FluxCarton struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	Length    string `json:"length"`
	Width     string `json:"width"`
	Height    string `json:"height"`
	MaxWeight string `json:"max_weight"`
}

// FluxAssignCartonRequest is the payload for POST /orders/carton.
type FluxAssignCartonRequest struct {
	OrderID   int     `json:"order_id"`
	CartonID  *string `json:"carton_id"`
	CreatedBy string  `json:"created_by"`
}

// FluxAssignCartonResponse is the response from POST /orders/carton.
type FluxAssignCartonResponse struct {
	Message string `json:"message"`
	ID      int    `json:"id"`
}
