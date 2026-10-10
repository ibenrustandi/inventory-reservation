package unit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

const reserveTarget = "/api/v1/inventory/reserve"

// AC-RES-04: runs the real handler and use case; the repository must not be
// reached.
func TestReserve_InvalidRequest(t *testing.T) {
	inventoryRepository := &mocks.InventoryRepository{
		ReserveFunc: func(context.Context, *domain.Reservation, time.Duration) (bool, error) {
			t.Error("repository must not be called for an invalid request")
			return false, errors.New("unreachable")
		},
	}
	app := newApp(usecase.NewInventoryUsecase(inventoryRepository, reservationTTL), &mocks.Pinger{})

	tests := []struct {
		name       string
		body       string
		wantFields []string
	}{
		{"empty body", ``, []string{"body"}},
		{"malformed JSON", `{"user_id":`, []string{"body"}},
		{"not an object", `[1,2]`, []string{"body"}},
		{"data after the object", `{"user_id":"usr_1","item_id":"item_4021","quantity":2} extra`, []string{"body"}},
		{"second object after the first", `{"user_id":"usr_1","item_id":"item_4021","quantity":2}{}`, []string{"body"}},
		{"all fields missing", `{}`, []string{"user_id", "item_id", "quantity"}},
		{"user_id missing", `{"item_id":"item_4021","quantity":2}`, []string{"user_id"}},
		{"item_id missing", `{"user_id":"usr_1","quantity":2}`, []string{"item_id"}},
		{"quantity missing", `{"user_id":"usr_1","item_id":"item_4021"}`, []string{"quantity"}},
		{"quantity null", `{"user_id":"usr_1","item_id":"item_4021","quantity":null}`, []string{"quantity"}},
		{"quantity zero", `{"user_id":"usr_1","item_id":"item_4021","quantity":0}`, []string{"quantity"}},
		{"quantity negative", `{"user_id":"usr_1","item_id":"item_4021","quantity":-1}`, []string{"quantity"}},
		{"quantity fractional", `{"user_id":"usr_1","item_id":"item_4021","quantity":2.5}`, []string{"quantity"}},
		{"quantity string", `{"user_id":"usr_1","item_id":"item_4021","quantity":"2"}`, []string{"quantity"}},
		{"quantity above maximum", `{"user_id":"usr_1","item_id":"item_4021","quantity":1001}`, []string{"quantity"}},
		{"user_id wrong type", `{"user_id":7,"item_id":"item_4021","quantity":2}`, []string{"user_id"}},
		{"user_id bad characters", `{"user_id":"usr 1","item_id":"item_4021","quantity":2}`, []string{"user_id"}},
		{"item_id too long", `{"user_id":"usr_1","item_id":"` + strings.Repeat("a", 65) + `","quantity":2}`, []string{"item_id"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doBody(t, app, http.MethodPost, reserveTarget, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body)
			}
			body := decodeError(t, rec)
			if body.Error.Code != exception.ErrInvalidRequest.Code {
				t.Fatalf("code = %q, want %q", body.Error.Code, exception.ErrInvalidRequest.Code)
			}
			if got := invalidFields(t, body); !slices.Equal(got, tt.wantFields) {
				t.Fatalf("invalid fields = %v, want %v", got, tt.wantFields)
			}
		})
	}
}

func TestReserve_RejectsOversizedBody(t *testing.T) {
	inventoryRepository := &mocks.InventoryRepository{
		ReserveFunc: func(context.Context, *domain.Reservation, time.Duration) (bool, error) {
			t.Error("repository must not be called for an oversized request")
			return false, errors.New("unreachable")
		},
	}
	app := newApp(usecase.NewInventoryUsecase(inventoryRepository, reservationTTL), &mocks.Pinger{})
	// Valid JSON all the way, so only its size is wrong.
	oversized := `{"user_id":"` + strings.Repeat("a", handler.MaxBodyBytes) + `","item_id":"item_4021","quantity":2}`

	tests := []struct {
		name string
		body io.Reader
	}{
		{"length declared", strings.NewReader(oversized)},
		// Wrapping the reader hides its length, as a chunked upload would.
		{"length not declared", struct{ io.Reader }{strings.NewReader(oversized)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, reserveTarget, tt.body))

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body)
			}
			if got := decodeError(t, rec).Error.Code; got != exception.ErrPayloadTooLarge.Code {
				t.Fatalf("code = %q, want %q", got, exception.ErrPayloadTooLarge.Code)
			}
		})
	}
}

func TestReserve_AcceptsQuantityBounds(t *testing.T) {
	inventoryRepository := &mocks.InventoryRepository{
		ReserveFunc: func(_ context.Context, reservation *domain.Reservation, _ time.Duration) (bool, error) {
			reservation.ExpiresAt = time.Date(2026, 7, 20, 16, 35, 0, 0, time.UTC)
			return true, nil
		},
	}
	app := newApp(usecase.NewInventoryUsecase(inventoryRepository, reservationTTL), &mocks.Pinger{})

	for _, quantity := range []string{"1", "1000"} {
		t.Run(quantity, func(t *testing.T) {
			body := `{"user_id":"usr_1","item_id":"item_4021","quantity":` + quantity + `}`

			rec := doBody(t, app, http.MethodPost, reserveTarget, body)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
			}
		})
	}
}

func TestReserveUsecase(t *testing.T) {
	request := func() *model.ReserveRequest {
		quantity := 2
		return &model.ReserveRequest{UserID: "usr_9981", ItemID: "item_4021", Quantity: &quantity}
	}

	t.Run("success", func(t *testing.T) {
		// A non-UTC time with a fraction, to check the response is UTC and
		// never rounded up.
		expiresAt := time.Date(2026, 7, 20, 23, 35, 0, 900_000_000, time.FixedZone("WIB", 7*60*60))
		var passed domain.Reservation
		var passedTTL time.Duration
		inventoryRepository := &mocks.InventoryRepository{
			ReserveFunc: func(_ context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error) {
				passed, passedTTL = *reservation, ttl
				reservation.ExpiresAt = expiresAt
				return true, nil
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		response, err := inventoryUsecase.Reserve(ctx, request())

		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		if passed.UserID != "usr_9981" || passed.ItemID != "item_4021" || passed.Quantity != 2 || passedTTL != reservationTTL {
			t.Fatalf("repository got %+v with ttl %s", passed, passedTTL)
		}
		if !strings.HasPrefix(passed.ID, "res_") || !domain.ValidID(passed.ID) {
			t.Fatalf("reservation ID = %q, want a valid res_ ID", passed.ID)
		}
		want := model.ReserveResponse{
			Status: "success", ReservationID: passed.ID, ItemID: "item_4021", Quantity: 2,
			ExpiresAt: "2026-07-20T16:35:00Z",
		}
		if *response != want {
			t.Fatalf("response = %+v, want %+v", *response, want)
		}
	})

	t.Run("reservation IDs are unique", func(t *testing.T) {
		seen := map[string]bool{}
		inventoryRepository := &mocks.InventoryRepository{
			ReserveFunc: func(_ context.Context, reservation *domain.Reservation, _ time.Duration) (bool, error) {
				if seen[reservation.ID] {
					t.Errorf("reservation ID %q was generated twice", reservation.ID)
				}
				seen[reservation.ID] = true
				return true, nil
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		for range 1000 {
			if _, err := inventoryUsecase.Reserve(ctx, request()); err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		}
	})

	notReserved := func(stock *domain.Stock, stockErr error) *mocks.InventoryRepository {
		return &mocks.InventoryRepository{
			ReserveFunc: func(context.Context, *domain.Reservation, time.Duration) (bool, error) {
				return false, nil
			},
			ReleaseDueForItemFunc: func(context.Context, string) (int, error) {
				return 0, nil
			},
			FindStockByItemIDFunc: func(context.Context, string) (*domain.Stock, error) {
				return stock, stockErr
			},
		}
	}

	t.Run("insufficient stock reports what is available", func(t *testing.T) {
		inventoryRepository := notReserved(&domain.Stock{ItemID: "item_4021", Total: 100, Reserved: 99}, nil)
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		if !errors.Is(err, exception.ErrInsufficientStock) {
			t.Fatalf("error = %v, want ErrInsufficientStock", err)
		}
		var e *exception.Error
		errors.As(err, &e)
		details, _ := e.Details.(map[string]any)
		if details["available_stock"] != 1 || details["requested"] != 2 {
			t.Fatalf("details = %v, want available_stock 1 and requested 2", e.Details)
		}
	})

	t.Run("unknown item", func(t *testing.T) {
		inventoryUsecase := usecase.NewInventoryUsecase(notReserved(nil, exception.ErrItemNotFound), reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		if !errors.Is(err, exception.ErrItemNotFound) {
			t.Fatalf("error = %v, want ErrItemNotFound", err)
		}
	})

	t.Run("repository error is returned unchanged", func(t *testing.T) {
		repoErr := errors.New("connection reset")
		inventoryRepository := &mocks.InventoryRepository{
			ReserveFunc: func(context.Context, *domain.Reservation, time.Duration) (bool, error) {
				return false, repoErr
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		_, err := inventoryUsecase.Reserve(ctx, request())

		if !errors.Is(err, repoErr) {
			t.Fatalf("error = %v, want %v", err, repoErr)
		}
	})
}
