// Package client provides the HTTP client for communicating with Flux WMS API.
// It handles request construction, retry logic, and response parsing.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/samber/oops"

	"github.com/anteraja/tetra-engine/internal/config"
	"github.com/anteraja/tetra-engine/internal/domain"
)

// FluxClient communicates with the Flux WMS API.
type FluxClient struct {
	httpClient *http.Client
	baseURL    string
	retryMax   int
	logger     *slog.Logger
}

// NewFluxClient creates a new FluxClient with configured timeout and retry.
func NewFluxClient(cfg *config.Config, logger *slog.Logger) *FluxClient {
	return &FluxClient{
		httpClient: &http.Client{
			Timeout: cfg.Flux.Timeout,
		},
		baseURL:  cfg.Flux.BaseURL,
		retryMax: cfg.Flux.RetryMax,
		logger:   logger.With(slog.String("component", "flux-client")),
	}
}

// GetOrders retrieves all orders from Flux WMS (GET /orders).
func (c *FluxClient) GetOrders(ctx context.Context) ([]domain.FluxOrder, error) {
	url := c.baseURL + "/orders"

	body, err := c.doGetWithRetry(ctx, url)
	if err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "orders").
			Wrapf(err, "fetching orders")
	}

	var orders []domain.FluxOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "orders").
			Wrapf(err, "decoding orders response")
	}

	return orders, nil
}

// GetOrderDetail retrieves order detail including items from Flux WMS (GET /orders/{id}).
func (c *FluxClient) GetOrderDetail(ctx context.Context, orderID int) (*domain.FluxOrderDetail, error) {
	url := fmt.Sprintf("%s/orders/%d", c.baseURL, orderID)

	body, err := c.doGetWithRetry(ctx, url)
	if err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "order-detail").
			With("order_id", orderID).
			Wrapf(err, "fetching order detail")
	}

	var detail domain.FluxOrderDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "order-detail").
			With("order_id", orderID).
			Wrapf(err, "decoding order detail response")
	}

	return &detail, nil
}

// GetCartons retrieves all cartons from Flux WMS (GET /cartons).
func (c *FluxClient) GetCartons(ctx context.Context) ([]domain.FluxCarton, error) {
	url := c.baseURL + "/cartons"

	body, err := c.doGetWithRetry(ctx, url)
	if err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "cartons").
			Wrapf(err, "fetching cartons")
	}

	var cartons []domain.FluxCarton
	if err := json.Unmarshal(body, &cartons); err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "cartons").
			Wrapf(err, "decoding cartons response")
	}

	return cartons, nil
}

// AssignCarton pushes a carton recommendation to Flux WMS (POST /orders/carton).
func (c *FluxClient) AssignCarton(ctx context.Context, req domain.FluxAssignCartonRequest) (*domain.FluxAssignCartonResponse, error) {
	url := c.baseURL + "/orders/carton"

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "assign-carton").
			Wrapf(err, "marshaling assign carton request")
	}

	body, err := c.doPostWithRetry(ctx, url, payload)
	if err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "assign-carton").
			With("order_id", req.OrderID).
			Wrapf(err, "assigning carton")
	}

	var resp domain.FluxAssignCartonResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, oops.
			In("flux-client").
			Tags("api", "assign-carton").
			Wrapf(err, "decoding assign carton response")
	}

	return &resp, nil
}

// doGetWithRetry performs a GET request with retry logic.
func (c *FluxClient) doGetWithRetry(ctx context.Context, url string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			c.logger.Warn("retrying request",
				slog.String("url", url),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryMax+1, lastErr)
}

// doPostWithRetry performs a POST request with retry logic.
func (c *FluxClient) doPostWithRetry(ctx context.Context, url string, payload []byte) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			c.logger.Warn("retrying request",
				slog.String("url", url),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryMax+1, lastErr)
}
