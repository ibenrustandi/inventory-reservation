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
	// Confirm completes a pending reservation. Confirming one that is
	// already confirmed succeeds again with the original time.
	Confirm(ctx context.Context, request *model.ConfirmRequest) (*model.ConfirmResponse, error)
	// ReleaseExpired returns the stock of every reservation that is past
	// its deadline to its item, and reports how many it released.
	ReleaseExpired(ctx context.Context) (int, error)
}

// DefaultSweepBatchSize keeps each release transaction short, so it never
// holds item rows for long while buyers are reserving.
const DefaultSweepBatchSize = 500

// InventoryUsecaseImpl implements InventoryUsecase.
type InventoryUsecaseImpl struct {
	InventoryRepository repository.InventoryRepository
	// ReservationTTL is how long a reservation holds stock.
	ReservationTTL time.Duration
	// SweepBatchSize is how many reservations ReleaseExpired releases per
	// database transaction.
	SweepBatchSize int
}

// NewInventoryUsecase returns the inventory use cases backed by the given
// repository.
func NewInventoryUsecase(inventoryRepo repository.InventoryRepository, reservationTTL time.Duration) InventoryUsecase {
	return &InventoryUsecaseImpl{
		InventoryRepository: inventoryRepo,
		ReservationTTL:      reservationTTL,
		SweepBatchSize:      DefaultSweepBatchSize,
	}
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
		reserved, err = uc.reserveAfterReleasingDue(ctx, reservation)
		if err != nil {
			return nil, err
		}
	}
	if !reserved {
		return nil, uc.explainNotReserved(ctx, reservation)
	}

	return mapper.ToReserveResponse(reservation), nil
}

// reserveAfterReleasingDue is the slow path of Reserve, taken only when the
// first attempt found too little stock. Holds on this item that are past
// their deadline are released, and if that freed anything the reserve is
// tried once more. It never loops. This keeps the sweeper's interval
// invisible to buyers: nobody is told "sold out" because of stock held by
// reservations that have already expired.
func (uc *InventoryUsecaseImpl) reserveAfterReleasingDue(ctx context.Context, reservation *domain.Reservation) (bool, error) {
	released, err := uc.InventoryRepository.ReleaseDueForItem(ctx, reservation.ItemID)
	if err != nil {
		return false, err
	}
	if released == 0 {
		return false, nil
	}
	return uc.InventoryRepository.Reserve(ctx, reservation, uc.ReservationTTL)
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

func (uc *InventoryUsecaseImpl) Confirm(ctx context.Context, request *model.ConfirmRequest) (*model.ConfirmResponse, error) {
	if fieldErr := validateID("reservation_id", request.ReservationID); fieldErr != nil {
		return nil, exception.NewInvalidRequest(*fieldErr)
	}

	reservation, confirmed, err := uc.InventoryRepository.Confirm(ctx, request.ReservationID)
	if err != nil {
		return nil, err
	}
	if confirmed {
		return mapper.ToConfirmResponse(reservation), nil
	}

	// Nothing was confirmed: find out why.
	reservation, err = uc.InventoryRepository.FindReservationByID(ctx, request.ReservationID)
	if errors.Is(err, exception.ErrReservationNotFound) {
		return nil, exception.ErrReservationNotFound.WithDetails(map[string]any{"reservation_id": request.ReservationID})
	}
	if err != nil {
		return nil, err
	}
	if reservation.Status == domain.ReservationConfirmed && reservation.ConfirmedAt != nil {
		// A repeated confirm is a safe retry: answer as the first one did.
		return mapper.ToConfirmResponse(reservation), nil
	}
	// A reservation only moves forward. Since it could not be confirmed and
	// is not confirmed, it is past its deadline, released or not.
	return nil, exception.ErrReservationExpired.WithDetails(map[string]any{
		"reservation_id": reservation.ID,
		"expires_at":     mapper.FormatTime(reservation.ExpiresAt),
	})
}

func (uc *InventoryUsecaseImpl) ReleaseExpired(ctx context.Context) (int, error) {
	total := 0
	for {
		released, err := uc.InventoryRepository.SweepDue(ctx, uc.SweepBatchSize)
		total += released
		if err != nil {
			return total, err
		}
		// A short batch means the backlog is drained, or another instance
		// is sweeping.
		if released < uc.SweepBatchSize {
			return total, nil
		}
	}
}

func validateReserveRequest(request *model.ReserveRequest) []exception.FieldError {
	var fieldErrs []exception.FieldError
	if fieldErr := validateID("user_id", request.UserID); fieldErr != nil {
		fieldErrs = append(fieldErrs, *fieldErr)
	}
	if fieldErr := validateID("item_id", request.ItemID); fieldErr != nil {
		fieldErrs = append(fieldErrs, *fieldErr)
	}
	switch {
	case request.Quantity == nil:
		fieldErrs = append(fieldErrs, exception.FieldError{Field: "quantity", Reason: "is required"})
	case !domain.ValidQuantity(*request.Quantity):
		fieldErrs = append(fieldErrs, exception.FieldError{
			Field:  "quantity",
			Reason: fmt.Sprintf("must be an integer from %d to %d", domain.MinQuantity, domain.MaxQuantity),
		})
	}
	return fieldErrs
}

// validateID checks a required ID field and returns what is wrong with it,
// or nil.
func validateID(field, value string) *exception.FieldError {
	if value == "" {
		return &exception.FieldError{Field: field, Reason: "is required"}
	}
	if !domain.ValidID(value) {
		return &exception.FieldError{Field: field, Reason: domain.IDFormat}
	}
	return nil
}
