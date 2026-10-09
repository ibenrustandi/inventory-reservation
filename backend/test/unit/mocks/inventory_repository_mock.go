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
	FindStockByItemIDFunc   func(ctx context.Context, itemID string) (*domain.Stock, error)
	ReserveFunc             func(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error)
	ConfirmFunc             func(ctx context.Context, reservationID string) (*domain.Reservation, bool, error)
	FindReservationByIDFunc func(ctx context.Context, reservationID string) (*domain.Reservation, error)
	SweepDueFunc            func(ctx context.Context, limit int) (int, error)
}

func (m *InventoryRepository) FindStockByItemID(ctx context.Context, itemID string) (*domain.Stock, error) {
	return m.FindStockByItemIDFunc(ctx, itemID)
}

func (m *InventoryRepository) Reserve(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error) {
	return m.ReserveFunc(ctx, reservation, ttl)
}

func (m *InventoryRepository) Confirm(ctx context.Context, reservationID string) (*domain.Reservation, bool, error) {
	return m.ConfirmFunc(ctx, reservationID)
}

func (m *InventoryRepository) FindReservationByID(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	return m.FindReservationByIDFunc(ctx, reservationID)
}

func (m *InventoryRepository) SweepDue(ctx context.Context, limit int) (int, error) {
	return m.SweepDueFunc(ctx, limit)
}
