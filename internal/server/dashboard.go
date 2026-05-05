// Package server — dashboard API handlers for the Tetra Engine monitoring UI.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DashboardStats holds aggregate statistics for the dashboard overview.
type DashboardStats struct {
	Orders   OrderStats    `json:"orders"`
	Cartons  CartonStats   `json:"cartons"`
	LastSync *SchedulerRun `json:"last_sync"`
}

// OrderStats holds order counts by status.
type OrderStats struct {
	Total       int `json:"total"`
	Synced      int `json:"synced"`
	Pending     int `json:"pending"`
	Recommended int `json:"recommended"`
	Pushed      int `json:"pushed"`
	Error       int `json:"error"`
}

// CartonStats holds carton summary.
type CartonStats struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

// SchedulerRun represents a single scheduler log for the API.
type SchedulerRun struct {
	ID               int64   `json:"id" db:"id"`
	SchedulerName    string  `json:"scheduler_name" db:"scheduler_name"`
	Status           string  `json:"status" db:"status"`
	RecordsProcessed *int    `json:"records_processed" db:"records_processed"`
	ErrorMessage     *string `json:"error_message" db:"error_message"`
	StartedAt        string  `json:"started_at" db:"started_at"`
	FinishedAt       *string `json:"finished_at" db:"finished_at"`
}

// OrderRow represents an order in the API response.
type OrderRow struct {
	ID            int64   `json:"id" db:"id"`
	Code          string  `json:"code" db:"code"`
	WarehouseID   string  `json:"warehouse_id" db:"warehouse_id"`
	Status        string  `json:"status" db:"status"`
	CartonID      *string `json:"carton_id" db:"carton_id"`
	FluxCreatedAt *string `json:"flux_created_at" db:"flux_created_at"`
	SyncedAt      *string `json:"synced_at" db:"synced_at"`
	RecommendedAt *string `json:"recommended_at" db:"recommended_at"`
	PushedAt      *string `json:"pushed_at" db:"pushed_at"`
	Reason        *string `json:"reason" db:"reason"`
	CreatedAt     string  `json:"created_at" db:"created_at"`
	UpdatedAt     string  `json:"updated_at" db:"updated_at"`
	ItemCount     int     `json:"item_count" db:"item_count"`
}

// OrderItemRow represents an order item in the API response.
type OrderItemRow struct {
	ID      int64   `json:"id" db:"id"`
	OrderID int64   `json:"order_id" db:"order_id"`
	SKU     string  `json:"sku" db:"sku"`
	SKUName *string `json:"sku_name" db:"sku_name"`
	Qty     int     `json:"qty" db:"qty"`
	Length  float64 `json:"length" db:"length"`
	Width   float64 `json:"width" db:"width"`
	Height  float64 `json:"height" db:"height"`
	Weight  float64 `json:"weight" db:"weight"`
}

// CartonRow represents a carton in the API response.
type CartonRow struct {
	ID        int64   `json:"id" db:"id"`
	Code      string  `json:"code" db:"code"`
	Length    float64 `json:"length" db:"length"`
	Width     float64 `json:"width" db:"width"`
	Height    float64 `json:"height" db:"height"`
	MaxWeight float64 `json:"max_weight" db:"max_weight"`
	IsActive  bool    `json:"is_active" db:"is_active"`
	Volume    float64 `json:"volume"`
	SyncedAt  *string `json:"synced_at" db:"synced_at"`
}

// OrderDetailResponse is a single order with its items.
type OrderDetailResponse struct {
	Order OrderRow       `json:"order"`
	Items []OrderItemRow `json:"items"`
}

// corsMiddleware adds CORS headers for local development.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

// handleDashboardStats returns aggregate statistics.
func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var stats DashboardStats

	// Order counts by status
	rows, err := s.db.QueryxContext(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'SYNCED') AS synced,
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending,
			COUNT(*) FILTER (WHERE status = 'RECOMMENDED') AS recommended,
			COUNT(*) FILTER (WHERE status = 'PUSHED') AS pushed,
			COUNT(*) FILTER (WHERE status = 'NO RECOMMENDATION') AS error
		FROM orders`)
	if err != nil {
		s.logger.Error("stats query failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database query failed"})
		return
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.StructScan(&stats.Orders); err != nil {
			s.logger.Error("stats scan failed", slog.Any("error", err))
		}
	}

	// Carton counts
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) AS total, COUNT(*) FILTER (WHERE is_active = TRUE) AS active FROM cartons`).
		Scan(&stats.Cartons.Total, &stats.Cartons.Active)
	if err != nil {
		s.logger.Error("carton stats failed", slog.Any("error", err))
	}

	// Last scheduler run
	var lastRun SchedulerRun
	err = s.db.QueryRowxContext(ctx,
		`SELECT id, scheduler_name, status,
		        records_processed, error_message,
		        TO_CHAR(started_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS started_at,
		        TO_CHAR(finished_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS finished_at
		 FROM scheduler_logs ORDER BY started_at DESC LIMIT 1`).
		StructScan(&lastRun)
	if err == nil {
		stats.LastSync = &lastRun
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleDashboardOrders returns all orders with item count, supporting filters.
func (s *Server) handleDashboardOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse filter parameters
	status := r.URL.Query().Get("status")
	date := r.URL.Query().Get("date")
	hour := r.URL.Query().Get("hour")

	query := `
		SELECT o.id, o.code, o.warehouse_id, o.status, o.carton_id, o.reason,
		       TO_CHAR(o.flux_created_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS flux_created_at,
		       TO_CHAR(o.synced_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS synced_at,
		       TO_CHAR(o.recommended_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS recommended_at,
		       TO_CHAR(o.pushed_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS pushed_at,
		       TO_CHAR(o.created_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS created_at,
		       TO_CHAR(o.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS updated_at,
		       COALESCE((SELECT COUNT(*) FROM order_items WHERE order_id = o.id), 0) AS item_count
		FROM orders o
		WHERE 1=1`

	var args []interface{}
	argCount := 1

	if status != "" && status != "ALL" {
		query += fmt.Sprintf(" AND o.status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if date != "" {
		query += fmt.Sprintf(" AND DATE(o.updated_at) = $%d", argCount)
		args = append(args, date)
		argCount++
	}

	if hour != "" {
		h, err := strconv.Atoi(hour)
		if err == nil {
			query += fmt.Sprintf(" AND EXTRACT(HOUR FROM o.updated_at) = $%d", argCount)
			args = append(args, h)
			argCount++
		}
	}

	query += " ORDER BY o.updated_at DESC"

	var orders []OrderRow
	err := s.db.SelectContext(ctx, &orders, query, args...)
	if err != nil {
		s.logger.Error("orders query failed", slog.Any("error", err), slog.String("query", query))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database query failed"})
		return
	}

	if orders == nil {
		orders = []OrderRow{}
	}

	writeJSON(w, http.StatusOK, orders)
}

// handleDashboardOrderDetail returns a single order with its items.
func (s *Server) handleDashboardOrderDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order id"})
		return
	}

	var order OrderRow
	err = s.db.QueryRowxContext(ctx, `
		SELECT o.id, o.code, o.warehouse_id, o.status, o.carton_id, o.reason,
		       TO_CHAR(o.flux_created_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS flux_created_at,
		       TO_CHAR(o.synced_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS synced_at,
		       TO_CHAR(o.recommended_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS recommended_at,
		       TO_CHAR(o.pushed_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS pushed_at,
		       TO_CHAR(o.created_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS created_at,
		       TO_CHAR(o.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS updated_at,
		       COALESCE((SELECT COUNT(*) FROM order_items WHERE order_id = o.id), 0) AS item_count
		FROM orders o
		WHERE o.id = $1`, orderID).StructScan(&order)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}

	var items []OrderItemRow
	err = s.db.SelectContext(ctx, &items, `
		SELECT oi.id, oi.order_id, oi.sku, p.name AS sku_name, oi.qty, oi.length, oi.width, oi.height, oi.weight
		FROM order_items oi
		LEFT JOIN products p ON oi.sku = p.sku
		WHERE oi.order_id = $1 ORDER BY oi.id`, orderID)
	if err != nil {
		s.logger.Error("order items query failed", slog.Any("error", err))
		items = []OrderItemRow{}
	}
	if items == nil {
		items = []OrderItemRow{}
	}

	writeJSON(w, http.StatusOK, OrderDetailResponse{Order: order, Items: items})
}

// handleDashboardCartons returns all cartons with computed volume.
func (s *Server) handleDashboardCartons(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	type cartonDB struct {
		ID        int64   `db:"id"`
		Code      string  `db:"code"`
		Length    float64 `db:"length"`
		Width     float64 `db:"width"`
		Height    float64 `db:"height"`
		MaxWeight float64 `db:"max_weight"`
		IsActive  bool    `db:"is_active"`
		SyncedAt  *string `db:"synced_at"`
	}

	var rows []cartonDB
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, code, length, width, height, max_weight, is_active,
		       TO_CHAR(synced_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS synced_at
		FROM cartons
		ORDER BY (length * width * height) ASC`)
	if err != nil {
		s.logger.Error("cartons query failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database query failed"})
		return
	}

	cartons := make([]CartonRow, 0, len(rows))
	for _, c := range rows {
		cartons = append(cartons, CartonRow{
			ID:        c.ID,
			Code:      c.Code,
			Length:    c.Length,
			Width:     c.Width,
			Height:    c.Height,
			MaxWeight: c.MaxWeight,
			IsActive:  c.IsActive,
			Volume:    c.Length * c.Width * c.Height,
			SyncedAt:  c.SyncedAt,
		})
	}

	writeJSON(w, http.StatusOK, cartons)
}

// handleDashboardLogs returns recent scheduler execution logs.
func (s *Server) handleDashboardLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	// Dynamic sorting
	sortBy := r.URL.Query().Get("sort_by")
	order := r.URL.Query().Get("order")

	// Validation to prevent SQL injection
	allowedSortFields := map[string]string{
		"started_at":  "started_at",
		"finished_at": "finished_at",
	}
	sortField, ok := allowedSortFields[sortBy]
	if !ok {
		sortField = "started_at" // default
	}

	if strings.ToLower(order) != "asc" {
		order = "DESC"
	} else {
		order = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT id, scheduler_name, status,
		       records_processed, error_message,
		       TO_CHAR(started_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS started_at,
		       TO_CHAR(finished_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS finished_at
		FROM scheduler_logs
		ORDER BY %s %s
		LIMIT $1`, sortField, order)

	var logs []SchedulerRun
	err := s.db.SelectContext(ctx, &logs, query, limit)
	if err != nil {
		s.logger.Error("logs query failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database query failed"})
		return
	}

	if logs == nil {
		logs = []SchedulerRun{}
	}

	writeJSON(w, http.StatusOK, logs)
}

// handleEngineStatus returns the current running status of the scheduler engine.
func (s *Server) handleEngineStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"running": s.scheduler.Status()})
}

// handleEngineStart starts the scheduler engine.
func (s *Server) handleEngineStart(w http.ResponseWriter, r *http.Request) {
	s.scheduler.Start()
	writeJSON(w, http.StatusOK, map[string]string{"message": "engine started"})
}

// handleEngineStop stops the scheduler engine.
func (s *Server) handleEngineStop(w http.ResponseWriter, r *http.Request) {
	s.scheduler.Stop()
	writeJSON(w, http.StatusOK, map[string]string{"message": "engine stopped"})
}

// handleEngineReset truncates all data in the database.
func (s *Server) handleEngineReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Stop the engine before resetting to prevent race conditions
	s.scheduler.Stop()
	
	_, err := s.db.ExecContext(ctx, "TRUNCATE TABLE orders, order_items, products, cartons, scheduler_logs RESTART IDENTITY CASCADE;")
	if err != nil {
		s.logger.Error("failed to reset database", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset database"})
		return
	}
	
	s.logger.Info("database reset successfully")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Database reset successfully. Engine stopped."})
}

// handleDashboardSettingsGet returns the current scheduler intervals.
func (s *Server) handleDashboardSettingsGet(w http.ResponseWriter, r *http.Request) {
	order, carton, rec, push := s.scheduler.GetIntervals()
	
	writeJSON(w, http.StatusOK, map[string]string{
		"order_sync_interval":    order.String(),
		"carton_sync_interval":   carton.String(),
		"recommendation_interval": rec.String(),
		"push_interval":          push.String(),
	})
}

// handleDashboardSettingsUpdate updates the scheduler intervals.
func (s *Server) handleDashboardSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderSyncInterval      string `json:"order_sync_interval"`
		CartonSyncInterval     string `json:"carton_sync_interval"`
		RecommendationInterval string `json:"recommendation_interval"`
		PushInterval           string `json:"push_interval"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	
	order, err1 := time.ParseDuration(req.OrderSyncInterval)
	carton, err2 := time.ParseDuration(req.CartonSyncInterval)
	rec, err3 := time.ParseDuration(req.RecommendationInterval)
	push, err4 := time.ParseDuration(req.PushInterval)
	
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid duration format (e.g. use '15m', '30s')"})
		return
	}
	
	if err := s.scheduler.UpdateIntervals(order, carton, rec, push); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.logger.Info("scheduler intervals updated",
		slog.String("order_sync", order.String()),
		slog.String("carton_sync", carton.String()),
		slog.String("recommendation", rec.String()),
		slog.String("push", push.String()),
	)
	
	writeJSON(w, http.StatusOK, map[string]string{"message": "Settings updated successfully"})
}
