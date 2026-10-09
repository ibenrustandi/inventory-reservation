// Package repository is the data access layer.
package repository

import (
	"context"
	"errors"
	"fmt"

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
