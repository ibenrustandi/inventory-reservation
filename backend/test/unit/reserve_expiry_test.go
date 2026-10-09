package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

// TestReserveUsecase_ReleasesDueHolds covers the order and number of
// repository calls in the reserve flow.
func TestReserveUsecase_ReleasesDueHolds(t *testing.T) {
	request := func() *model.ReserveRequest {
		quantity := 2
		return &model.ReserveRequest{UserID: "usr_1", ItemID: "item_4021", Quantity: &quantity}
	}

	// repo builds a repository that records its calls. Reserve returns the
	// given results in turn; ReleaseDueForItem returns released.
	repo := func(calls *[]string, released int, releaseErr error, reserveResults ...bool) *mocks.InventoryRepository {
		return &mocks.InventoryRepository{
			ReserveFunc: func(context.Context, *domain.Reservation, time.Duration) (bool, error) {
				*calls = append(*calls, "reserve")
				if len(reserveResults) == 0 {
					t.Fatal("Reserve was called more often than the flow allows")
				}
				result := reserveResults[0]
				reserveResults = reserveResults[1:]
				return result, nil
			},
			ReleaseDueForItemFunc: func(_ context.Context, itemID string) (int, error) {
				*calls = append(*calls, "release:"+itemID)
				return released, releaseErr
			},
			FindStockByItemIDFunc: func(_ context.Context, itemID string) (*domain.Stock, error) {
				*calls = append(*calls, "find")
				return &domain.Stock{ItemID: itemID, Total: 2, Reserved: 2}, nil
			},
		}
	}

	t.Run("fast path makes one call", func(t *testing.T) {
		var calls []string
		inventoryUsecase := usecase.NewInventoryUsecase(repo(&calls, 0, nil, true), reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		require.NoError(t, err)
		require.Equal(t, []string{"reserve"}, calls)
	})

	t.Run("releases due holds of the item, then reserves once more", func(t *testing.T) {
		var calls []string
		inventoryUsecase := usecase.NewInventoryUsecase(repo(&calls, 1, nil, false, true), reservationTTL)

		response, err := inventoryUsecase.Reserve(ctx, request())

		require.NoError(t, err)
		require.Equal(t, "success", response.Status)
		require.Equal(t, []string{"reserve", "release:item_4021", "reserve"}, calls)
	})

	t.Run("does not retry when nothing was released", func(t *testing.T) {
		var calls []string
		inventoryUsecase := usecase.NewInventoryUsecase(repo(&calls, 0, nil, false), reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		require.ErrorIs(t, err, exception.ErrInsufficientStock)
		require.Equal(t, []string{"reserve", "release:item_4021", "find"}, calls)
	})

	t.Run("retries once only, never loops", func(t *testing.T) {
		var calls []string
		inventoryUsecase := usecase.NewInventoryUsecase(repo(&calls, 3, nil, false, false), reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		require.ErrorIs(t, err, exception.ErrInsufficientStock)
		require.Equal(t, []string{"reserve", "release:item_4021", "reserve", "find"}, calls)
	})

	t.Run("a failed release is returned and nothing is retried", func(t *testing.T) {
		var calls []string
		releaseErr := errors.New("connection reset")
		inventoryUsecase := usecase.NewInventoryUsecase(repo(&calls, 0, releaseErr, false), reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		require.ErrorIs(t, err, releaseErr)
		require.Equal(t, []string{"reserve", "release:item_4021"}, calls)
	})
}
