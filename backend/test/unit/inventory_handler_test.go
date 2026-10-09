package unit

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

const stockTarget = "/api/v1/inventory/stock?item_id=item_4021"

// AC-STK-03: runs the real handler and use case; the repository must not be
// reached.
func TestStock_InvalidRequest(t *testing.T) {
	inventoryRepository := &mocks.InventoryRepository{
		FindStockByItemIDFunc: func(context.Context, string) (*domain.Stock, error) {
			t.Error("repository must not be called for an invalid request")
			return nil, errors.New("unreachable")
		},
	}
	app := newApp(usecase.NewInventoryUsecase(inventoryRepository, reservationTTL), &mocks.Pinger{})

	tests := []struct {
		name   string
		target string
	}{
		{"missing", "/api/v1/inventory/stock"},
		{"empty", "/api/v1/inventory/stock?item_id="},
		{"space", "/api/v1/inventory/stock?item_id=" + url.QueryEscape("item 4021")},
		{"symbols", "/api/v1/inventory/stock?item_id=" + url.QueryEscape("item_4021';--")},
		{"too long", "/api/v1/inventory/stock?item_id=" + strings.Repeat("a", 65)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, app, http.MethodGet, tt.target)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			body := decodeError(t, rec)
			if body.Error.Code != exception.ErrInvalidRequest.Code {
				t.Fatalf("code = %q, want %q", body.Error.Code, exception.ErrInvalidRequest.Code)
			}
			details, _ := body.Error.Details.(map[string]any)
			fields, _ := details["fields"].([]any)
			if len(fields) != 1 {
				t.Fatalf("details.fields = %v, want one entry", body.Error.Details)
			}
			if field := fields[0].(map[string]any)["field"]; field != "item_id" {
				t.Fatalf("field = %v, want item_id", field)
			}
		})
	}
}

func TestStock_MapsUsecaseResult(t *testing.T) {
	stockReturning := func(response *model.StockResponse, err error) *mocks.InventoryUsecase {
		return &mocks.InventoryUsecase{
			StockFunc: func(context.Context, *model.GetStockRequest) (*model.StockResponse, error) {
				return response, err
			},
		}
	}

	t.Run("success", func(t *testing.T) {
		uc := stockReturning(&model.StockResponse{
			ItemID: "item_4021", TotalStock: 100, ReservedStock: 15, AvailableStock: 85,
		}, nil)

		rec := do(t, newApp(uc, &mocks.Pinger{}), http.MethodGet, stockTarget)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		want := `{"item_id":"item_4021","total_stock":100,"reserved_stock":15,"available_stock":85}`
		if got := rec.Body.String(); got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
	})

	t.Run("item not found", func(t *testing.T) {
		uc := stockReturning(nil, exception.ErrItemNotFound)

		rec := do(t, newApp(uc, &mocks.Pinger{}), http.MethodGet, stockTarget)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
		if got := decodeError(t, rec).Error.Code; got != exception.ErrItemNotFound.Code {
			t.Fatalf("code = %q, want %q", got, exception.ErrItemNotFound.Code)
		}
	})

	t.Run("unexpected error is not leaked", func(t *testing.T) {
		const secret = "password authentication failed for user inventory"
		uc := stockReturning(nil, errors.New(secret))

		rec := do(t, newApp(uc, &mocks.Pinger{}), http.MethodGet, stockTarget)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if got := decodeError(t, rec).Error.Code; got != exception.ErrInternalServerError.Code {
			t.Fatalf("code = %q, want %q", got, exception.ErrInternalServerError.Code)
		}
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("body leaks the internal error: %s", rec.Body)
		}
	})

	t.Run("error with no status mapping becomes a 500 envelope", func(t *testing.T) {
		uc := stockReturning(nil, &exception.Error{Code: "NOT_MAPPED", Message: "secret detail"})

		rec := do(t, newApp(uc, &mocks.Pinger{}), http.MethodGet, stockTarget)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if got := decodeError(t, rec).Error.Code; got != exception.ErrInternalServerError.Code {
			t.Fatalf("code = %q, want %q", got, exception.ErrInternalServerError.Code)
		}
		if strings.Contains(rec.Body.String(), "secret detail") {
			t.Fatalf("body leaks the unmapped error: %s", rec.Body)
		}
	})

	t.Run("panic becomes a 500 envelope", func(t *testing.T) {
		uc := &mocks.InventoryUsecase{
			StockFunc: func(context.Context, *model.GetStockRequest) (*model.StockResponse, error) {
				panic("boom")
			},
		}

		rec := do(t, newApp(uc, &mocks.Pinger{}), http.MethodGet, stockTarget)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if got := decodeError(t, rec).Error.Code; got != exception.ErrInternalServerError.Code {
			t.Fatalf("code = %q, want %q", got, exception.ErrInternalServerError.Code)
		}
	})
}
