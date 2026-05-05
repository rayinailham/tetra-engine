package service

import (
	"testing"

	"github.com/anteraja/tetra-engine/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateOrderDimensions(t *testing.T) {
	tests := []struct {
		name        string
		items       []domain.OrderItem
		wantVolume  int
		wantWeight  int
	}{
		{
			name:       "empty items",
			items:      []domain.OrderItem{},
			wantVolume: 0,
			wantWeight: 0,
		},
		{
			name: "single item qty 1",
			items: []domain.OrderItem{
				{Length: 150, Width: 80, Height: 30, Weight: 500, Qty: 1}, // 15cm x 8cm x 3cm
			},
			wantVolume: 360000, // 150 * 80 * 30
			wantWeight: 500,
		},
		{
			name: "single item qty 2",
			items: []domain.OrderItem{
				{Length: 100, Width: 100, Height: 100, Weight: 1000, Qty: 2},
			},
			wantVolume: 2000000,
			wantWeight: 2000,
		},
		{
			name: "multiple items",
			items: []domain.OrderItem{
				{Length: 150, Width: 80, Height: 30, Weight: 500, Qty: 1},
				{Length: 180, Width: 100, Height: 20, Weight: 200, Qty: 1},
			},
			wantVolume: 720000, // 360,000 + 360,000
			wantWeight: 700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVolume, gotWeight := CalculateOrderDimensions(tt.items)
			assert.Equal(t, tt.wantVolume, gotVolume, "volume mismatch")
			assert.Equal(t, tt.wantWeight, gotWeight, "weight mismatch")
		})
	}
}

func TestFindBestCarton(t *testing.T) {
	// Cartons sorted by volume ascending (mm)
	cartons := []domain.Carton{
		{ID: 1, Code: "CB01S", Length: 200, Width: 160, Height: 60, MaxWeight: 5000},      // vol=1,920,000
		{ID: 2, Code: "CB02M", Length: 250, Width: 100, Height: 120, MaxWeight: 8000},     // vol=3,000,000
		{ID: 3, Code: "CB03L", Length: 250, Width: 180, Height: 150, MaxWeight: 12000},    // vol=6,750,000
		{ID: 4, Code: "CB06XL1", Length: 290, Width: 210, Height: 100, MaxWeight: 15000},  // vol=6,090,000
		{ID: 5, Code: "CB05XXL", Length: 400, Width: 220, Height: 100, MaxWeight: 20000},  // vol=8,800,000
	}

	tests := []struct {
		name     string
		items    []domain.OrderItem
		wantCode string
		wantNil  bool
	}{
		{
			name:     "small item fits in smallest carton",
			items:    []domain.OrderItem{{Length: 150, Width: 80, Height: 30, Weight: 500, Qty: 1}},
			wantCode: "CB01S",
		},
		{
			name:     "medium item needs CB02M",
			items:    []domain.OrderItem{{Length: 100, Width: 100, Height: 100, Weight: 1000, Qty: 2}},
			wantCode: "CB02M",
		},
		{
			name:     "volume fits CB01S but weight exceeds — needs bigger carton",
			items:    []domain.OrderItem{{Length: 120, Width: 80, Height: 50, Weight: 3000, Qty: 2}},
			wantCode: "CB02M",
		},
		{
			name:     "exact fit on volume boundary",
			items:    []domain.OrderItem{{Length: 200, Width: 160, Height: 60, Weight: 5000, Qty: 1}},
			wantCode: "CB01S",
		},
		{
			name:     "item too heavy for any carton",
			items:    []domain.OrderItem{{Length: 10, Width: 10, Height: 10, Weight: 50000, Qty: 1}},
			wantNil:  true,
		},
		{
			name:     "volume fits but dimensions require a larger carton",
			items:    []domain.OrderItem{{Length: 300, Width: 100, Height: 20, Weight: 1000, Qty: 1}},
			wantCode: "CB05XXL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindBestCarton(cartons, tt.items)

			if tt.wantNil {
				assert.Nil(t, result, "expected no suitable carton")
			} else {
				require.NotNil(t, result, "expected a suitable carton")
				assert.Equal(t, tt.wantCode, result.Code)
			}
		})
	}
}
