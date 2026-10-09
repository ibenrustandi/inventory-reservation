// Package usecase holds the business logic of each operation.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	// Reserve holds stock for a user until the reservation is confirmed or
	// expires.
	Reserve(ctx context.Context, request *model.ReserveRequest) (*model.ReserveResponse, error)
}

// InventoryUsecaseImpl implements InventoryUsecase.
type InventoryUsecaseImpl struct {
	InventoryRepository repository.InventoryRepository
	// ReservationTTL is how long a reservation holds stock.
	ReservationTTL time.Duration
}

// NewInventoryUsecase returns the inventory use cases backed by the given
// repository.
func NewInventoryUsecase(inventoryRepo repository.InventoryRepository, reservationTTL time.Duration) InventoryUsecase {
	return &InventoryUsecaseImpl{InventoryRepository: inventoryRepo, ReservationTTL: reservationTTL}
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

func (uc *InventoryUsecaseImpl) Reserve(ctx context.Context, request *model.ReserveRequest) (*model.ReserveResponse, error) {
	if fieldErrs := validateReserveRequest(request); len(fieldErrs) > 0 {
		return nil, exception.NewInvalidRequest(fieldErrs...)
	}

	reservation := &domain.Reservation{
		ID:       domain.NewReservationID(),
		UserID:   request.UserID,
		ItemID:   request.ItemID,
		Quantity: *request.Quantity,
	}

	reserved, err := uc.InventoryRepository.Reserve(ctx, reservation, uc.ReservationTTL)
	if err != nil {
		return nil, err
	}
	if !reserved {
		return nil, uc.explainNotReserved(ctx, reservation)
	}

	return mapper.ToReserveResponse(reservation), nil
}

// explainNotReserved works out why a reserve changed nothing: the item
// does not exist, or it has too little available stock.
func (uc *InventoryUsecaseImpl) explainNotReserved(ctx context.Context, reservation *domain.Reservation) error {
	stock, err := uc.InventoryRepository.FindStockByItemID(ctx, reservation.ItemID)
	if errors.Is(err, exception.ErrItemNotFound) {
		return exception.ErrItemNotFound.WithDetails(map[string]any{"item_id": reservation.ItemID})
	}
	if err != nil {
		return err
	}
	return exception.ErrInsufficientStock.WithDetails(map[string]any{
		"available_stock": stock.Available(),
		"requested":       reservation.Quantity,
	})
}

func validateReserveRequest(request *model.ReserveRequest) []model.FieldError {
	var fieldErrs []model.FieldError
	if fieldErr := validateID("user_id", request.UserID); fieldErr != nil {
		fieldErrs = append(fieldErrs, *fieldErr)
	}
	if fieldErr := validateID("item_id", request.ItemID); fieldErr != nil {
		fieldErrs = append(fieldErrs, *fieldErr)
	}
	switch {
	case request.Quantity == nil:
		fieldErrs = append(fieldErrs, model.FieldError{Field: "quantity", Reason: "is required"})
	case !domain.ValidQuantity(*request.Quantity):
		fieldErrs = append(fieldErrs, model.FieldError{
			Field:  "quantity",
			Reason: fmt.Sprintf("must be an integer from %d to %d", domain.MinQuantity, domain.MaxQuantity),
		})
	}
	return fieldErrs
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
