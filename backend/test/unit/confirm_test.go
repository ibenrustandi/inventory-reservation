package unit

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

const confirmTarget = "/api/v1/inventory/confirm"

// AC-CON-06: runs the real handler and use case; the repository must not be
// reached.
func TestConfirm_InvalidRequest(t *testing.T) {
	inventoryRepository := &mocks.InventoryRepository{
		ConfirmFunc: func(context.Context, string) (*domain.Reservation, bool, error) {
			t.Error("repository must not be called for an invalid request")
			return nil, false, errors.New("unreachable")
		},
	}
	app := newApp(usecase.NewInventoryUsecase(inventoryRepository, reservationTTL), &mocks.Pinger{})

	tests := []struct {
		name       string
		body       string
		wantFields []string
	}{
		{"empty body", ``, []string{"body"}},
		{"malformed JSON", `{"reservation_id"`, []string{"body"}},
		{"missing", `{}`, []string{"reservation_id"}},
		{"empty", `{"reservation_id":""}`, []string{"reservation_id"}},
		{"null", `{"reservation_id":null}`, []string{"reservation_id"}},
		{"wrong type", `{"reservation_id":883291}`, []string{"reservation_id"}},
		{"bad characters", `{"reservation_id":"res 883291"}`, []string{"reservation_id"}},
		{"too long", `{"reservation_id":"` + strings.Repeat("a", 65) + `"}`, []string{"reservation_id"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doBody(t, app, http.MethodPost, confirmTarget, tt.body)

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

func TestConfirmUsecase(t *testing.T) {
	request := &model.ConfirmRequest{ReservationID: "res_883291"}
	confirmedAt := time.Date(2026, 7, 20, 16, 32, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 7, 20, 16, 35, 0, 0, time.UTC)

	// notConfirmed is a repository whose Confirm changes nothing and whose
	// lookup returns the given reservation.
	notConfirmed := func(found *domain.Reservation, findErr error) *mocks.InventoryRepository {
		return &mocks.InventoryRepository{
			ConfirmFunc: func(context.Context, string) (*domain.Reservation, bool, error) {
				return nil, false, nil
			},
			FindReservationByIDFunc: func(context.Context, string) (*domain.Reservation, error) {
				return found, findErr
			},
		}
	}

	t.Run("success", func(t *testing.T) {
		inventoryRepository := &mocks.InventoryRepository{
			ConfirmFunc: func(_ context.Context, reservationID string) (*domain.Reservation, bool, error) {
				return &domain.Reservation{
					ID: reservationID, Status: domain.ReservationConfirmed, ConfirmedAt: &confirmedAt,
				}, true, nil
			},
			FindReservationByIDFunc: func(context.Context, string) (*domain.Reservation, error) {
				t.Error("a successful confirm must not look the reservation up again")
				return nil, errors.New("unreachable")
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		response, err := inventoryUsecase.Confirm(ctx, request)

		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		want := model.ConfirmResponse{Status: "success", ReservationID: "res_883291", ConfirmedAt: "2026-07-20T16:32:00Z"}
		if *response != want {
			t.Fatalf("response = %+v, want %+v", *response, want)
		}
	})

	t.Run("already confirmed answers with the original time", func(t *testing.T) {
		inventoryRepository := notConfirmed(&domain.Reservation{
			ID: "res_883291", Status: domain.ReservationConfirmed, ExpiresAt: expiresAt, ConfirmedAt: &confirmedAt,
		}, nil)
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		response, err := inventoryUsecase.Confirm(ctx, request)

		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		if response.ConfirmedAt != "2026-07-20T16:32:00Z" {
			t.Fatalf("confirmed_at = %s, want the original time", response.ConfirmedAt)
		}
	})

	for _, status := range []domain.ReservationStatus{domain.ReservationPending, domain.ReservationExpired} {
		t.Run("not confirmable and "+string(status)+" is expired", func(t *testing.T) {
			inventoryRepository := notConfirmed(&domain.Reservation{
				ID: "res_883291", Status: status, ExpiresAt: expiresAt,
			}, nil)
			inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

			_, err := inventoryUsecase.Confirm(ctx, request)

			if !errors.Is(err, exception.ErrReservationExpired) {
				t.Fatalf("error = %v, want ErrReservationExpired", err)
			}
			var e *exception.Error
			errors.As(err, &e)
			if details, _ := e.Details.(map[string]any); details["expires_at"] != "2026-07-20T16:35:00Z" {
				t.Fatalf("details = %v, want expires_at 2026-07-20T16:35:00Z", e.Details)
			}
		})
	}

	t.Run("unknown reservation", func(t *testing.T) {
		inventoryUsecase := usecase.NewInventoryUsecase(notConfirmed(nil, exception.ErrReservationNotFound), reservationTTL)

		_, err := inventoryUsecase.Confirm(ctx, request)

		if !errors.Is(err, exception.ErrReservationNotFound) {
			t.Fatalf("error = %v, want ErrReservationNotFound", err)
		}
	})

	t.Run("repository error is returned unchanged", func(t *testing.T) {
		repoErr := errors.New("connection reset")
		inventoryRepository := &mocks.InventoryRepository{
			ConfirmFunc: func(context.Context, string) (*domain.Reservation, bool, error) {
				return nil, false, repoErr
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		_, err := inventoryUsecase.Confirm(ctx, request)

		if !errors.Is(err, repoErr) {
			t.Fatalf("error = %v, want %v", err, repoErr)
		}
	})
}
