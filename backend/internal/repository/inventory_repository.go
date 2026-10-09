// Package repository is the data access layer.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
)

// InventoryRepository is the data access the inventory use cases need.
type InventoryRepository interface {
	// FindStockByItemID returns the stock of an item, or
	// exception.ErrItemNotFound.
	FindStockByItemID(ctx context.Context, itemID string) (*domain.Stock, error)
	// Reserve holds reservation.Quantity units of the item and stores the
	// reservation, atomically, if enough stock is available. It reports
	// whether the reservation was made and, if so, sets its Status and
	// ExpiresAt. It returns false both when stock is insufficient and when
	// the item does not exist.
	Reserve(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error)
}

// InventoryRepositoryImpl is the PostgreSQL implementation.
type InventoryRepositoryImpl struct {
	DB *pgxpool.Pool
}

// NewInventoryRepository returns a repository that runs its queries on the
// given pool.
func NewInventoryRepository(db *pgxpool.Pool) InventoryRepository {
	return &InventoryRepositoryImpl{DB: db}
}

func (r *InventoryRepositoryImpl) FindStockByItemID(ctx context.Context, itemID string) (*domain.Stock, error) {
	stock := &domain.Stock{ItemID: itemID}
	err := r.DB.QueryRow(ctx,
		`SELECT item_stock, item_stock_reserved FROM inv_item WHERE item_id = $1`,
		itemID,
	).Scan(&stock.Total, &stock.Reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, exception.ErrItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find stock by item id: %w", err)
	}
	return stock, nil
}

// reserveSQL is the whole oversell protection. The UPDATE takes the item's
// row lock and re-checks availability against the latest committed row, so
// two concurrent requests can never both take the last units. The INSERT
// runs only if the UPDATE changed a row, in the same statement.
const reserveSQL = `
WITH upd AS (
  UPDATE inv_item
  SET item_stock_reserved = item_stock_reserved + $3, updated_at = now()
  WHERE item_id = $2 AND item_stock - item_stock_reserved >= $3
  RETURNING item_id
)
INSERT INTO inv_reservation (reservation_id, user_id, item_id,
                             reservation_quantity, reservation_status, reservation_expires_date)
SELECT $1, $4, item_id, $3, 'PENDING', now() + $5::interval FROM upd
RETURNING reservation_expires_date`

func (r *InventoryRepositoryImpl) Reserve(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error) {
	var expiresAt time.Time
	err := r.DB.QueryRow(ctx, reserveSQL,
		reservation.ID, reservation.ItemID, reservation.Quantity, reservation.UserID, ttl,
	).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reserve: %w", err)
	}
	reservation.Status = domain.ReservationPending
	reservation.ExpiresAt = expiresAt
	return true, nil
}
