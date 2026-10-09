// Package mocks holds hand-written test doubles for the service's
// interfaces.
package mocks

import (
	"context"
	"time"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
)

// InventoryRepository is a test double for repository.InventoryRepository.
// Each field stands in for the method of the same name.
type InventoryRepository struct {
	FindStockByItemIDFunc func(ctx context.Context, itemID string) (*domain.Stock, error)
	ReserveFunc           func(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error)
}

func (m *InventoryRepository) FindStockByItemID(ctx context.Context, itemID string) (*domain.Stock, error) {
	return m.FindStockByItemIDFunc(ctx, itemID)
}

func (m *InventoryRepository) Reserve(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error) {
	return m.ReserveFunc(ctx, reservation, ttl)
}
