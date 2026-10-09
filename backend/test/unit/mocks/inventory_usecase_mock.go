package mocks

import (
	"context"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
)

// InventoryUsecase is a test double for usecase.InventoryUsecase.
type InventoryUsecase struct {
	StockFunc func(ctx context.Context, request *model.GetStockRequest) (*model.StockResponse, error)
}

func (m *InventoryUsecase) Stock(ctx context.Context, request *model.GetStockRequest) (*model.StockResponse, error) {
	return m.StockFunc(ctx, request)
}
