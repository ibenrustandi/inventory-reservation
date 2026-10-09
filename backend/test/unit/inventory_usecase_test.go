package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

var ctx = context.Background()

func TestStockUsecase(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		inventoryRepository := &mocks.InventoryRepository{
			FindStockByItemIDFunc: func(_ context.Context, itemID string) (*domain.Stock, error) {
				return &domain.Stock{ItemID: itemID, Total: 100, Reserved: 15}, nil
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		response, err := inventoryUsecase.Stock(ctx, &model.GetStockRequest{ItemID: "item_4021"})

		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		want := model.StockResponse{ItemID: "item_4021", TotalStock: 100, ReservedStock: 15, AvailableStock: 85}
		if *response != want {
			t.Fatalf("response = %+v, want %+v", *response, want)
		}
	})

	t.Run("item not found carries the item id", func(t *testing.T) {
		inventoryRepository := &mocks.InventoryRepository{
			FindStockByItemIDFunc: func(context.Context, string) (*domain.Stock, error) {
				return nil, exception.ErrItemNotFound
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		_, err := inventoryUsecase.Stock(ctx, &model.GetStockRequest{ItemID: "item_9999"})

		if !errors.Is(err, exception.ErrItemNotFound) {
			t.Fatalf("error = %v, want ErrItemNotFound", err)
		}
		var e *exception.Error
		errors.As(err, &e)
		if details, _ := e.Details.(map[string]any); details["item_id"] != "item_9999" {
			t.Fatalf("details = %v, want item_id item_9999", e.Details)
		}
		if exception.ErrItemNotFound.Details != nil {
			t.Fatal("the shared ErrItemNotFound was modified")
		}
	})

	t.Run("repository error is returned unchanged", func(t *testing.T) {
		repoErr := errors.New("connection reset")
		inventoryRepository := &mocks.InventoryRepository{
			FindStockByItemIDFunc: func(context.Context, string) (*domain.Stock, error) {
				return nil, repoErr
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		_, err := inventoryUsecase.Stock(ctx, &model.GetStockRequest{ItemID: "item_4021"})

		if !errors.Is(err, repoErr) {
			t.Fatalf("error = %v, want %v", err, repoErr)
		}
	})
}
