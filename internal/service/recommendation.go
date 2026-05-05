// Package service implements the core business logic for the Tetra Engine,
// including the carton recommendation algorithm and data synchronization.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/samber/oops"
	"golang.org/x/sync/errgroup"

	"github.com/anteraja/tetra-engine/internal/client"
	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/domain"
	"github.com/anteraja/tetra-engine/internal/observability"
	"github.com/anteraja/tetra-engine/internal/repository"
)

// RecommendationService handles the core business logic.
type RecommendationService struct {
	orderRepo  *repository.OrderRepository
	cartonRepo *repository.CartonRepository
	logRepo    *repository.SchedulerLogRepository
	outboxRepo *repository.OutboxRepository
	fluxClient *client.FluxClient
	cfg        *config.Config
	metrics    *observability.Metrics
	logger     *slog.Logger
}

// NewRecommendationService creates a new RecommendationService.
func NewRecommendationService(
	orderRepo *repository.OrderRepository,
	cartonRepo *repository.CartonRepository,
	logRepo *repository.SchedulerLogRepository,
	outboxRepo *repository.OutboxRepository,
	fluxClient *client.FluxClient,
	cfg *config.Config,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *RecommendationService {
	return &RecommendationService{
		orderRepo:  orderRepo,
		cartonRepo: cartonRepo,
		logRepo:    logRepo,
		outboxRepo: outboxRepo,
		fluxClient: fluxClient,
		cfg:        cfg,
		metrics:    metrics,
		logger:     logger.With(slog.String("component", "recommendation-service")),
	}
}

// SyncOrders retrieves orders from Flux and syncs them to the local database.
// This is Job 1: Order Retrieval Sync.
func (s *RecommendationService) SyncOrders(ctx context.Context) error {
	logID, err := s.logRepo.StartLog(ctx, "order_retrieval")
	if err != nil {
		return oops.In("service").Wrapf(err, "starting scheduler log")
	}

	processed := 0
	syncErr := s.doSyncOrders(ctx, &processed)

	status := domain.SchedulerStatusSuccess
	var errMsg *string
	if syncErr != nil {
		status = domain.SchedulerStatusFailed
		msg := syncErr.Error()
		errMsg = &msg
	}

	if finishErr := s.logRepo.FinishLog(ctx, logID, status, processed, errMsg); finishErr != nil {
		s.logger.Error("failed to finish scheduler log", slog.Any("error", finishErr))
	}

	return syncErr
}

func (s *RecommendationService) doSyncOrders(ctx context.Context, processed *int) error {
	// Step 1: Fetch orders list from Flux
	fluxOrders, err := s.fluxClient.GetOrders(ctx)
	if err != nil {
		return oops.In("service").Wrapf(err, "fetching orders from flux")
	}

	s.logger.Info("fetched orders from flux", slog.Int("count", len(fluxOrders)))

	// Step 2: Parallel fetch details and upsert
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(10) // Concurrency limit to protect Flux API and DB connections

	var mu sync.Mutex
	for _, fo := range fluxOrders {
		orderInfo := fo // capture loop variable
		g.Go(func() error {
			// Fetch order detail with items
			detail, err := s.fluxClient.GetOrderDetail(gCtx, orderInfo.ID)
			if err != nil {
				s.logger.Error("failed to fetch order detail",
					slog.Int("flux_order_id", orderInfo.ID),
					slog.Any("error", err),
				)
				return nil // continue other orders
			}

			var fluxCreatedAt *time.Time
			if orderInfo.CreatedAt != "" {
				parsed, parseErr := time.Parse(time.RFC3339, orderInfo.CreatedAt)
				if parseErr != nil {
					s.logger.Warn("invalid flux created_at, using nil",
						slog.String("order_code", orderInfo.Code),
						slog.String("created_at", orderInfo.CreatedAt),
						slog.Any("error", parseErr),
					)
				} else {
					fluxCreatedAt = &parsed
				}
			}

			now := time.Now()
			order := &domain.Order{
				FluxID:        orderInfo.ID,
				Code:          orderInfo.Code,
				WarehouseID:   orderInfo.WarehouseID,
				Status:        domain.OrderStatusSynced,
				FluxCreatedAt: fluxCreatedAt,
				SyncedAt:      &now,
			}

			// Upsert order
			localOrderID, err := s.orderRepo.UpsertOrder(gCtx, order)
			if err != nil {
				s.logger.Error("failed to upsert order",
					slog.String("order_code", orderInfo.Code),
					slog.Any("error", err),
				)
				return nil
			}

			// Convert and insert items/products
			items := make([]domain.OrderItem, 0, len(detail.Details))
			products := make([]domain.Product, 0, len(detail.Details))
			seenProducts := make(map[string]struct{}, len(detail.Details))
			for _, di := range detail.Details {
				length, parseErr := parseDecimalToScaledInt(di.Length, 10)
				if parseErr != nil {
					s.logger.Warn("invalid item length, skipping item",
						slog.String("order_code", orderInfo.Code),
						slog.String("sku", di.SKU),
						slog.String("length", di.Length),
						slog.Any("error", parseErr),
					)
					continue
				}

				width, parseErr := parseDecimalToScaledInt(di.Width, 10)
				if parseErr != nil {
					s.logger.Warn("invalid item width, skipping item",
						slog.String("order_code", orderInfo.Code),
						slog.String("sku", di.SKU),
						slog.String("width", di.Width),
						slog.Any("error", parseErr),
					)
					continue
				}

				height, parseErr := parseDecimalToScaledInt(di.Height, 10)
				if parseErr != nil {
					s.logger.Warn("invalid item height, skipping item",
						slog.String("order_code", orderInfo.Code),
						slog.String("sku", di.SKU),
						slog.String("height", di.Height),
						slog.Any("error", parseErr),
					)
					continue
				}

				weight, parseErr := parseDecimalToScaledInt(di.Weight, 1000)
				if parseErr != nil {
					s.logger.Warn("invalid item weight, skipping item",
						slog.String("order_code", orderInfo.Code),
						slog.String("sku", di.SKU),
						slog.String("weight", di.Weight),
						slog.Any("error", parseErr),
					)
					continue
				}

				productName := di.SKUName
				if _, seen := seenProducts[di.SKU]; !seen {
					products = append(products, domain.Product{
						SKU:  di.SKU,
						Name: &productName,
					})
					seenProducts[di.SKU] = struct{}{}
				}

				items = append(items, domain.OrderItem{
					SKU:    di.SKU,
					Qty:    di.Qty,
					Length: length, // cm to mm
					Width:  width,
					Height: height,
					Weight: weight, // kg to grams
				})
			}

			if len(items) == 0 {
				s.logger.Warn("order has no valid items after parsing, skipping items insert",
					slog.String("order_code", orderInfo.Code),
					slog.Int("flux_order_id", orderInfo.ID),
				)
				return nil
			}

			if err := s.orderRepo.InsertOrderItems(gCtx, localOrderID, items, products); err != nil {
				s.logger.Error("failed to insert order items",
					slog.Int64("order_id", localOrderID),
					slog.Any("error", err),
				)
				return nil
			}

			mu.Lock()
			*processed++
			mu.Unlock()
			s.logger.Debug("synced order",
				slog.String("code", orderInfo.Code),
				slog.Int64("local_id", localOrderID),
			)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return oops.In("service").Wrapf(err, "syncing orders in parallel")
	}

	// Step 3: Transition SYNCED → PENDING
	pendingCount, err := s.orderRepo.MarkSyncedAsPending(ctx)
	if err != nil {
		return oops.In("service").Wrapf(err, "marking synced orders as pending")
	}

	s.logger.Info("order sync complete",
		slog.Int("synced", *processed),
		slog.Int64("marked_pending", pendingCount),
	)

	return nil
}

// SyncCartons retrieves carton master data from Flux and syncs to local database.
// This is Job 2: Carton Master Sync.
func (s *RecommendationService) SyncCartons(ctx context.Context) error {
	logID, err := s.logRepo.StartLog(ctx, "carton_sync")
	if err != nil {
		return oops.In("service").Wrapf(err, "starting scheduler log")
	}

	processed := 0
	syncErr := s.doSyncCartons(ctx, &processed)

	status := domain.SchedulerStatusSuccess
	var errMsg *string
	if syncErr != nil {
		status = domain.SchedulerStatusFailed
		msg := syncErr.Error()
		errMsg = &msg
	}

	if finishErr := s.logRepo.FinishLog(ctx, logID, status, processed, errMsg); finishErr != nil {
		s.logger.Error("failed to finish scheduler log", slog.Any("error", finishErr))
	}

	return syncErr
}

func (s *RecommendationService) doSyncCartons(ctx context.Context, processed *int) error {
	fluxCartons, err := s.fluxClient.GetCartons(ctx)
	if err != nil {
		return oops.In("service").Wrapf(err, "fetching cartons from flux")
	}

	s.logger.Info("fetched cartons from flux", slog.Int("count", len(fluxCartons)))

	for _, fc := range fluxCartons {
		length, parseErr := parseDecimalToScaledInt(fc.Length, 10)
		if parseErr != nil {
			s.logger.Warn("invalid carton length, skipping carton",
				slog.String("carton_code", fc.Code),
				slog.String("length", fc.Length),
				slog.Any("error", parseErr),
			)
			continue
		}

		width, parseErr := parseDecimalToScaledInt(fc.Width, 10)
		if parseErr != nil {
			s.logger.Warn("invalid carton width, skipping carton",
				slog.String("carton_code", fc.Code),
				slog.String("width", fc.Width),
				slog.Any("error", parseErr),
			)
			continue
		}

		height, parseErr := parseDecimalToScaledInt(fc.Height, 10)
		if parseErr != nil {
			s.logger.Warn("invalid carton height, skipping carton",
				slog.String("carton_code", fc.Code),
				slog.String("height", fc.Height),
				slog.Any("error", parseErr),
			)
			continue
		}

		maxWeight, parseErr := parseDecimalToScaledInt(fc.MaxWeight, 1000)
		if parseErr != nil {
			s.logger.Warn("invalid carton max_weight, skipping carton",
				slog.String("carton_code", fc.Code),
				slog.String("max_weight", fc.MaxWeight),
				slog.Any("error", parseErr),
			)
			continue
		}

		carton := &domain.Carton{
			FluxID:    fc.ID,
			Code:      fc.Code,
			Length:    length, // cm to mm
			Width:     width,
			Height:    height,
			MaxWeight: maxWeight, // kg to grams
			IsActive:  true,
		}

		if err := s.cartonRepo.UpsertCarton(ctx, carton); err != nil {
			s.logger.Error("failed to upsert carton",
				slog.String("carton_code", fc.Code),
				slog.Any("error", err),
			)
			continue
		}

		*processed++
	}

	s.logger.Info("carton sync complete", slog.Int("processed", *processed))
	return nil
}

// ProcessRecommendations runs the recommendation algorithm on PENDING orders.
// This is Job 3: Carton Recommendation.
func (s *RecommendationService) ProcessRecommendations(ctx context.Context) error {
	logID, err := s.logRepo.StartLog(ctx, "carton_recommendation")
	if err != nil {
		return oops.In("service").Wrapf(err, "starting scheduler log")
	}

	processed := 0
	recErr := s.doProcessRecommendations(ctx, &processed)

	status := domain.SchedulerStatusSuccess
	var errMsg *string
	if recErr != nil {
		status = domain.SchedulerStatusFailed
		msg := recErr.Error()
		errMsg = &msg
	}

	if finishErr := s.logRepo.FinishLog(ctx, logID, status, processed, errMsg); finishErr != nil {
		s.logger.Error("failed to finish scheduler log", slog.Any("error", finishErr))
	}

	return recErr
}

func (s *RecommendationService) doProcessRecommendations(ctx context.Context, processed *int) error {
	pendingBefore, err := s.orderRepo.CountByStatus(ctx, domain.OrderStatusPending)
	if err != nil {
		return oops.In("service").Wrapf(err, "counting pending orders")
	}
	s.metrics.SetPendingOrders(pendingBefore)

	if pendingBefore == 0 {
		s.logger.Info("no pending orders to process")
		return nil
	}

	// Get active cartons (pre-sorted by volume ascending)
	cartons, err := s.cartonRepo.GetActiveCartons(ctx)
	if err != nil {
		return oops.In("service").Wrapf(err, "fetching active cartons")
	}

	if len(cartons) == 0 {
		s.logger.Warn("no active cartons available for recommendation")
		return nil
	}

	s.logger.Info("processing recommendations in batches",
		slog.Int("pending_orders", pendingBefore),
		slog.Int("batch_size", s.cfg.Scheduler.RecommendationBatchSize),
		slog.Int("available_cartons", len(cartons)),
	)

	lastID := int64(0)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		orders, pageErr := s.orderRepo.GetOrdersByStatusAfterID(ctx, domain.OrderStatusPending, lastID, s.cfg.Scheduler.RecommendationBatchSize)
		if pageErr != nil {
			return oops.In("service").Wrapf(pageErr, "fetching pending orders page")
		}
		if len(orders) == 0 {
			break
		}

		orderIDs := make([]int64, len(orders))
		for i, o := range orders {
			orderIDs[i] = o.ID
		}

		allItems, itemsErr := s.orderRepo.GetOrderItemsByOrderIDs(ctx, orderIDs)
		if itemsErr != nil {
			return oops.In("service").Wrapf(itemsErr, "bulk fetching order items")
		}

		itemsMap := make(map[int64][]domain.OrderItem)
		for _, item := range allItems {
			itemsMap[item.OrderID] = append(itemsMap[item.OrderID], item)
		}

		for _, order := range orders {
			recordCtx, cancel := context.WithTimeout(ctx, s.cfg.Scheduler.RecordTimeout)
			err := s.processRecommendationOrder(recordCtx, order, cartons, itemsMap[order.ID], processed)
			cancel()
			if err != nil {
				s.logger.Error("failed processing recommendation order",
					slog.Int64("order_id", order.ID),
					slog.String("order_code", order.Code),
					slog.Any("error", err),
				)
			}

			lastID = order.ID
		}
	}

	pendingAfter, countErr := s.orderRepo.CountByStatus(ctx, domain.OrderStatusPending)
	if countErr == nil {
		s.metrics.SetPendingOrders(pendingAfter)
	}

	s.logger.Info("recommendation processing complete", slog.Int("processed", *processed))
	return nil
}

func (s *RecommendationService) processRecommendationOrder(ctx context.Context, order domain.Order, cartons []domain.Carton, items []domain.OrderItem, processed *int) error {
	if len(items) == 0 {
		s.logger.Warn("order has no items, skipping", slog.Int64("order_id", order.ID))
		return nil
	}

	totalVolume, totalWeight := CalculateOrderDimensions(items)
	carton := FindBestCarton(cartons, totalVolume, totalWeight)
	if carton == nil {
		s.logger.Warn("no suitable carton found for order",
			slog.Int64("order_id", order.ID),
			slog.String("order_code", order.Code),
			slog.Int("total_volume_mm3", totalVolume),
			slog.Int("total_weight_g", totalWeight),
		)
		reason := fmt.Sprintf("No carton fits Volume: %d mm³, Weight: %d g", totalVolume, totalWeight)
		if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusError, nil, &reason); err != nil {
			return oops.In("service").With("order_id", order.ID).Wrapf(err, "marking order as no recommendation")
		}
		return nil
	}

	cartonID := carton.ID
	reason := fmt.Sprintf("Fits in %s (Vol: %d mm³, MaxWt: %d g)", carton.Code, carton.Volume(), carton.MaxWeight)
	if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusRecommended, &cartonID, &reason); err != nil {
		return oops.In("service").With("order_id", order.ID).Wrapf(err, "updating order with recommendation")
	}

	*processed++
	s.logger.Debug("recommended carton for order",
		slog.String("order_code", order.Code),
		slog.String("carton_code", carton.Code),
		slog.Int("order_volume", totalVolume),
		slog.Int("carton_volume", carton.Volume()),
		slog.Int("order_weight", totalWeight),
		slog.Int("carton_max_weight", carton.MaxWeight),
	)

	return nil
}

// PushRecommendations pushes recommended cartons to Flux WMS.
// This is Job 4: Carton Push.
func (s *RecommendationService) PushRecommendations(ctx context.Context) error {
	logID, err := s.logRepo.StartLog(ctx, "carton_push")
	if err != nil {
		return oops.In("service").Wrapf(err, "starting scheduler log")
	}

	processed := 0
	pushErr := s.doPushRecommendations(ctx, &processed)

	status := domain.SchedulerStatusSuccess
	var errMsg *string
	if pushErr != nil {
		status = domain.SchedulerStatusFailed
		msg := pushErr.Error()
		errMsg = &msg
	}

	if finishErr := s.logRepo.FinishLog(ctx, logID, status, processed, errMsg); finishErr != nil {
		s.logger.Error("failed to finish scheduler log", slog.Any("error", finishErr))
	}

	return pushErr
}

func (s *RecommendationService) doPushRecommendations(ctx context.Context, processed *int) error {
	enqueued, err := s.enqueuePushIntents(ctx)
	if err != nil {
		return err
	}

	localCartons, err := s.cartonRepo.GetActiveCartons(ctx)
	if err != nil {
		return oops.In("service").Wrapf(err, "fetching local cartons for push")
	}
	cartonMap := make(map[int64]domain.Carton)
	for _, c := range localCartons {
		cartonMap[c.ID] = c
	}

	s.logger.Info("delivering push outbox in batches",
		slog.Int("batch_size", s.cfg.Scheduler.PushBatchSize),
		slog.Int("enqueued", enqueued),
	)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		rows, listErr := s.outboxRepo.ListPendingPushes(ctx, s.cfg.Scheduler.PushBatchSize)
		if listErr != nil {
			return oops.In("service").Wrapf(listErr, "listing pending push outbox")
		}

		if len(rows) == 0 {
			break
		}

		s.deliverOutboxBatch(ctx, rows, cartonMap, processed)
	}

	s.logger.Info("push processing complete", slog.Int("processed", *processed))
	return nil
}

func (s *RecommendationService) enqueuePushIntents(ctx context.Context) (int, error) {
	enqueued := 0
	lastID := int64(0)

	for {
		orders, err := s.orderRepo.GetOrdersByStatusesAfterID(
			ctx,
			[]string{domain.OrderStatusRecommended, domain.OrderStatusError},
			lastID,
			s.cfg.Scheduler.PushBatchSize,
		)
		if err != nil {
			return 0, oops.In("service").Wrapf(err, "fetching push candidates page")
		}

		if len(orders) == 0 {
			break
		}

		for _, order := range orders {
			fluxCartonID, resolveErr := s.resolveFluxCartonIDForPush(ctx, order)
			if resolveErr != nil {
				s.logger.Error("failed to resolve flux carton id for push",
					slog.Int64("order_id", order.ID),
					slog.String("order_code", order.Code),
					slog.Any("error", resolveErr),
				)
				lastID = order.ID
				continue
			}

			recordCtx, cancel := context.WithTimeout(ctx, s.cfg.Scheduler.RecordTimeout)
			inserted, enqueueErr := s.outboxRepo.EnqueuePushIntent(recordCtx, order, fluxCartonID, s.cfg.CreatedBy)
			cancel()
			if enqueueErr != nil {
				s.logger.Error("failed to enqueue push outbox",
					slog.Int64("order_id", order.ID),
					slog.String("order_code", order.Code),
					slog.Any("error", enqueueErr),
				)
			} else if inserted {
				enqueued++
			}

			lastID = order.ID
		}
	}

	return enqueued, nil
}

func (s *RecommendationService) resolveFluxCartonIDForPush(ctx context.Context, order domain.Order) (*string, error) {
	if order.Status == domain.OrderStatusError {
		none := "0"
		return &none, nil
	}

	if order.CartonID == nil {
		return nil, fmt.Errorf("recommended order missing carton_id")
	}

	localCarton, err := s.cartonRepo.GetCartonByID(ctx, *order.CartonID)
	if err != nil {
		return nil, oops.In("service").With("carton_id", *order.CartonID).Wrapf(err, "querying carton by id")
	}
	if localCarton == nil {
		return nil, fmt.Errorf("local carton %d not found", *order.CartonID)
	}

	idStr := fmt.Sprintf("%d", localCarton.FluxID)
	return &idStr, nil
}

func (s *RecommendationService) deliverOutboxBatch(ctx context.Context, rows []domain.PushOutbox, cartonMap map[int64]domain.Carton, processed *int) {
	for _, row := range rows {
		recordCtx, cancel := context.WithTimeout(ctx, s.cfg.Scheduler.RecordTimeout)
		err := s.deliverOutboxRecord(recordCtx, row, cartonMap)
		cancel()

		if err != nil {
			backoff := nextRetryBackoff(row.AttemptCount)
			s.metrics.IncPushRetry()
			if retryErr := s.outboxRepo.MarkRetry(ctx, row.ID, err.Error(), backoff); retryErr != nil {
				s.logger.Error("failed to mark outbox retry",
					slog.Int64("outbox_id", row.ID),
					slog.Any("error", retryErr),
				)
			}
			continue
		}

		*processed++
	}
}

func (s *RecommendationService) deliverOutboxRecord(ctx context.Context, row domain.PushOutbox, cartonMap map[int64]domain.Carton) error {
	if row.FluxCartonID == nil {
		return fmt.Errorf("outbox row %d missing flux_carton_id", row.ID)
	}

	req := domain.FluxAssignCartonRequest{
		OrderID:   row.FluxOrderID,
		CartonID:  row.FluxCartonID,
		CreatedBy: row.CreatedBy,
	}

	resp, err := s.fluxClient.AssignCarton(ctx, req)
	if err != nil {
		return oops.In("service").With("outbox_id", row.ID).Wrapf(err, "pushing carton assignment")
	}

	var cartonID *int64
	if *row.FluxCartonID != "0" {
		for id, carton := range cartonMap {
			if fmt.Sprintf("%d", carton.FluxID) == *row.FluxCartonID {
				idCopy := id
				cartonID = &idCopy
				break
			}
		}
	}

	if err := s.outboxRepo.MarkDelivered(ctx, row.ID, row.OrderID, cartonID); err != nil {
		return oops.In("service").With("outbox_id", row.ID).Wrapf(err, "marking outbox delivery success")
	}

	s.logger.Info("pushed carton assignment",
		slog.Int64("outbox_id", row.ID),
		slog.Int64("order_id", row.OrderID),
		slog.Int("flux_response_id", resp.ID),
		slog.String("message", resp.Message),
	)

	return nil
}

func nextRetryBackoff(attempt int) time.Duration {
	base := 5 * time.Second
	if attempt <= 0 {
		return base
	}

	exp := attempt
	if exp > 6 {
		exp = 6
	}

	backoff := base * time.Duration(1<<exp)
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}

	jitter := time.Duration(rand.Int63n(int64(2 * time.Second)))
	return backoff + jitter
}

// CalculateOrderDimensions calculates total volume (mm³) and weight (g) for all items.
// Volume = sum of (length × width × height × qty) for each item.
// Weight = sum of (weight × qty) for each item.
func CalculateOrderDimensions(items []domain.OrderItem) (totalVolume int, totalWeight int) {
	for _, item := range items {
		itemVolume := item.Length * item.Width * item.Height * item.Qty
		totalVolume += itemVolume
		totalWeight += item.Weight * item.Qty
	}
	return totalVolume, totalWeight
}

// FindBestCarton selects the smallest carton that can hold the given volume and weight.
// Cartons must be pre-sorted by volume ascending.
// Returns nil if no suitable carton is found.
func FindBestCarton(cartons []domain.Carton, totalVolume int, totalWeight int) *domain.Carton {
	for i := range cartons {
		cartonVolume := cartons[i].Volume()

		if cartonVolume >= totalVolume && cartons[i].MaxWeight >= totalWeight {
			return &cartons[i]
		}
	}
	return nil
}

func parseDecimalToScaledInt(raw string, scale int) (int, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}

	return int(math.Round(v * float64(scale))), nil
}
