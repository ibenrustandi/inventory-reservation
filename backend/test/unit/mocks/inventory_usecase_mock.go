package mocks

import (
	"context"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
)

// InventoryUsecase is a test double for usecase.InventoryUsecase.
type InventoryUsecase struct {
	StockFunc   func(ctx context.Context, request *model.GetStockRequest) (*model.StockResponse, error)
	ReserveFunc func(ctx context.Context, request *model.ReserveRequest) (*model.ReserveResponse, error)
	ConfirmFunc func(ctx context.Context, request *model.ConfirmRequest) (*model.ConfirmResponse, error)
}

func (m *InventoryUsecase) Stock(ctx context.Context, request *model.GetStockRequest) (*model.StockResponse, error) {
	return m.StockFunc(ctx, request)
}

func (m *InventoryUsecase) Reserve(ctx context.Context, request *model.ReserveRequest) (*model.ReserveResponse, error) {
	return m.ReserveFunc(ctx, request)
}

func (m *InventoryUsecase) Confirm(ctx context.Context, request *model.ConfirmRequest) (*model.ConfirmResponse, error) {
	return m.ConfirmFunc(ctx, request)
}
