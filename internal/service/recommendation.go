// Package service implements the core business logic for the Tetra Engine,
// including the carton recommendation algorithm and data synchronization.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/samber/oops"

	"github.com/anteraja/tetra-engine/internal/client"
	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/domain"
	"github.com/anteraja/tetra-engine/internal/repository"
)

// RecommendationService handles the core business logic.
type RecommendationService struct {
	orderRepo  *repository.OrderRepository
	cartonRepo *repository.CartonRepository
	logRepo    *repository.SchedulerLogRepository
	fluxClient *client.FluxClient
	cfg        *config.Config
	logger     *slog.Logger
}

// NewRecommendationService creates a new RecommendationService.
func NewRecommendationService(
	orderRepo *repository.OrderRepository,
	cartonRepo *repository.CartonRepository,
	logRepo *repository.SchedulerLogRepository,
	fluxClient *client.FluxClient,
	cfg *config.Config,
	logger *slog.Logger,
) *RecommendationService {
	return &RecommendationService{
		orderRepo:  orderRepo,
		cartonRepo: cartonRepo,
		logRepo:    logRepo,
		fluxClient: fluxClient,
		cfg:        cfg,
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

	// Step 2: For each order, fetch detail and upsert
	for _, fo := range fluxOrders {
		// Fetch order detail with items
		detail, err := s.fluxClient.GetOrderDetail(ctx, fo.ID)
		if err != nil {
			s.logger.Error("failed to fetch order detail",
				slog.Int("flux_order_id", fo.ID),
				slog.Any("error", err),
			)
			continue
		}

		// Parse timestamps
		fluxCreatedAt, _ := time.Parse(time.RFC3339, fo.CreatedAt)

		now := time.Now()
		order := &domain.Order{
			Code:          fo.Code,
			WarehouseID:   fo.WarehouseID,
			Status:        domain.OrderStatusSynced,
			FluxCreatedAt: &fluxCreatedAt,
			SyncedAt:      &now,
		}

		// Upsert order
		localOrderID, err := s.orderRepo.UpsertOrder(ctx, order)
		if err != nil {
			s.logger.Error("failed to upsert order",
				slog.String("order_code", fo.Code),
				slog.Any("error", err),
			)
			continue
		}

		// Convert and insert items/products
		items := make([]domain.OrderItem, 0, len(detail.Details))
		products := make([]domain.Product, 0, len(detail.Details))
		for _, di := range detail.Details {
			length, _ := strconv.ParseFloat(di.Length, 64)
			width, _ := strconv.ParseFloat(di.Width, 64)
			height, _ := strconv.ParseFloat(di.Height, 64)
			weight, _ := strconv.ParseFloat(di.Weight, 64)

			productName := di.SKUName
			products = append(products, domain.Product{
				SKU:  di.SKU,
				Name: &productName,
			})

			items = append(items, domain.OrderItem{
				SKU:    di.SKU,
				Qty:    di.Qty,
				Length: int(length * 10), // cm to mm
				Width:  int(width * 10),
				Height: int(height * 10),
				Weight: int(weight),      // grams
			})
		}

		if err := s.orderRepo.InsertOrderItems(ctx, localOrderID, items, products); err != nil {
			s.logger.Error("failed to insert order items",
				slog.Int64("order_id", localOrderID),
				slog.Any("error", err),
			)
			continue
		}

		*processed++
		s.logger.Debug("synced order",
			slog.String("code", fo.Code),
			slog.Int64("local_id", localOrderID),
		)
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
		length, _ := strconv.ParseFloat(fc.Length, 64)
		width, _ := strconv.ParseFloat(fc.Width, 64)
		height, _ := strconv.ParseFloat(fc.Height, 64)
		maxWeight, _ := strconv.ParseFloat(fc.MaxWeight, 64)

		carton := &domain.Carton{
			Code:      fc.Code,
			Length:    int(length * 10),     // cm to mm
			Width:     int(width * 10),
			Height:    int(height * 10),
			MaxWeight: int(maxWeight * 1000), // kg to grams
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
	// Get PENDING orders
	orders, err := s.orderRepo.GetOrdersByStatus(ctx, domain.OrderStatusPending)
	if err != nil {
		return oops.In("service").Wrapf(err, "fetching pending orders")
	}

	if len(orders) == 0 {
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

	s.logger.Info("processing recommendations",
		slog.Int("pending_orders", len(orders)),
		slog.Int("available_cartons", len(cartons)),
	)

	for _, order := range orders {
		// Get items for this order
		items, err := s.orderRepo.GetOrderItemsByOrderID(ctx, order.ID)
		if err != nil {
			s.logger.Error("failed to fetch order items",
				slog.Int64("order_id", order.ID),
				slog.Any("error", err),
			)
			continue
		}

		// Calculate total volume and weight (all in int)
		totalVolume, totalWeight := CalculateOrderDimensions(items)

		// Find the smallest carton that fits
		carton := FindBestCarton(cartons, totalVolume, totalWeight)

		if carton == nil {
			s.logger.Warn("no suitable carton found for order",
				slog.Int64("order_id", order.ID),
				slog.String("order_code", order.Code),
				slog.Int("total_volume_mm3", totalVolume),
				slog.Int("total_weight_g", totalWeight),
			)
			if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusError, nil); err != nil {
				s.logger.Error("failed to mark order as error", slog.Any("error", err))
			}
			continue
		}

		// Update order with recommended carton ID
		cartonID := carton.ID
		if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusRecommended, &cartonID); err != nil {
			s.logger.Error("failed to update order with recommendation",
				slog.Int64("order_id", order.ID),
				slog.Any("error", err),
			)
			continue
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
	}

	s.logger.Info("recommendation processing complete", slog.Int("processed", *processed))
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
	// Get RECOMMENDED orders
	orders, err := s.orderRepo.GetOrdersByStatus(ctx, domain.OrderStatusRecommended)
	if err != nil {
		return oops.In("service").Wrapf(err, "fetching recommended orders")
	}

	if len(orders) == 0 {
		s.logger.Info("no recommended orders to push")
		return nil
	}

	s.logger.Info("pushing recommendations to flux", slog.Int("count", len(orders)))

	for _, order := range orders {
		if order.CartonID == nil {
			s.logger.Error("recommended order missing carton_id",
				slog.Int64("order_id", order.ID),
				slog.String("order_code", order.Code),
			)
			continue
		}

		// Look up local carton to get the code string
		localCarton, err := s.cartonRepo.GetCartonByID(ctx, *order.CartonID)
		if err != nil || localCarton == nil {
			s.logger.Error("failed to find local carton by id", slog.Int64("carton_id", *order.CartonID))
			continue
		}

		// Look up the original Flux order by code to get the Flux numeric ID
		fluxOrders, err := s.fluxClient.GetOrders(ctx)
		if err != nil {
			return oops.In("service").Wrapf(err, "fetching flux orders for push")
		}

		var fluxOrderID int
		for _, fo := range fluxOrders {
			if fo.Code == order.Code {
				fluxOrderID = fo.ID
				break
			}
		}

		if fluxOrderID == 0 {
			s.logger.Error("could not find flux order ID for code",
				slog.String("order_code", order.Code),
			)
			continue
		}

		// Look up carton by code to get its Flux ID
		fluxCartons, err := s.fluxClient.GetCartons(ctx)
		if err != nil {
			return oops.In("service").Wrapf(err, "fetching flux cartons for push")
		}

		var fluxCartonID string
		for _, fc := range fluxCartons {
			if fc.Code == localCarton.Code {
				fluxCartonID = fmt.Sprintf("%d", fc.ID)
				break
			}
		}

		if fluxCartonID == "" {
			s.logger.Error("could not find flux carton ID for code",
				slog.String("carton_code", localCarton.Code),
			)
			continue
		}

		// Push to Flux
		req := domain.FluxAssignCartonRequest{
			OrderID:   fluxOrderID,
			CartonID:  fluxCartonID,
			CreatedBy: s.cfg.CreatedBy,
		}

		resp, err := s.fluxClient.AssignCarton(ctx, req)
		if err != nil {
			s.logger.Error("failed to push carton assignment",
				slog.String("order_code", order.Code),
				slog.Any("error", err),
			)
			continue
		}

		// Mark as PUSHED
		if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusPushed, nil); err != nil {
			s.logger.Error("failed to mark order as pushed",
				slog.Int64("order_id", order.ID),
				slog.Any("error", err),
			)
			continue
		}

		*processed++
		s.logger.Info("pushed carton assignment",
			slog.String("order_code", order.Code),
			slog.Int("flux_response_id", resp.ID),
			slog.String("message", resp.Message),
		)
	}

	s.logger.Info("push processing complete", slog.Int("processed", *processed))
	return nil
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
