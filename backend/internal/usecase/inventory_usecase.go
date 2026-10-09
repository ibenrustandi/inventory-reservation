// Package usecase holds the business logic of each operation.
package usecase

import (
	"context"
	"errors"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model/mapper"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/repository"
)

// InventoryUsecase is the set of inventory operations.
type InventoryUsecase interface {
	// Stock returns the stock breakdown of an item.
	Stock(ctx context.Context, request *model.GetStockRequest) (*model.StockResponse, error)
}

// InventoryUsecaseImpl implements InventoryUsecase.
type InventoryUsecaseImpl struct {
	InventoryRepository repository.InventoryRepository
}

// NewInventoryUsecase returns the inventory use cases backed by the given
// repository.
func NewInventoryUsecase(inventoryRepo repository.InventoryRepository) InventoryUsecase {
	return &InventoryUsecaseImpl{InventoryRepository: inventoryRepo}
}

func (uc *InventoryUsecaseImpl) Stock(ctx context.Context, request *model.GetStockRequest) (*model.StockResponse, error) {
	if fieldErr := validateID("item_id", request.ItemID); fieldErr != nil {
		return nil, exception.NewInvalidRequest(*fieldErr)
	}

	stock, err := uc.InventoryRepository.FindStockByItemID(ctx, request.ItemID)
	if errors.Is(err, exception.ErrItemNotFound) {
		return nil, exception.ErrItemNotFound.WithDetails(map[string]any{"item_id": request.ItemID})
	}
	if err != nil {
		return nil, err
	}

	return mapper.ToStockResponse(stock), nil
}

// validateID checks a required ID field and returns what is wrong with it,
// or nil.
func validateID(field, value string) *model.FieldError {
	if value == "" {
		return &model.FieldError{Field: field, Reason: "is required"}
	}
	if !domain.ValidID(value) {
		return &model.FieldError{Field: field, Reason: domain.IDFormat}
	}
	return nil
}
