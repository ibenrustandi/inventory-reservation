// Package mapper converts domain types into API response structures.
package mapper

import (
	"time"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
)

// ToStockResponse converts a domain stock into its API response.
func ToStockResponse(stock *domain.Stock) *model.StockResponse {
	return &model.StockResponse{
		ItemID:         stock.ItemID,
		TotalStock:     stock.Total,
		ReservedStock:  stock.Reserved,
		AvailableStock: stock.Available(),
	}
}

// ToReserveResponse converts a reservation into its API response.
func ToReserveResponse(reservation *domain.Reservation) *model.ReserveResponse {
	return &model.ReserveResponse{
		Status:        "success",
		ReservationID: reservation.ID,
		ItemID:        reservation.ItemID,
		Quantity:      reservation.Quantity,
		ExpiresAt:     FormatTime(reservation.ExpiresAt),
	}
}

// ToConfirmResponse converts a confirmed reservation into its API response.
func ToConfirmResponse(reservation *domain.Reservation) *model.ConfirmResponse {
	return &model.ConfirmResponse{
		Status:        "success",
		ReservationID: reservation.ID,
		ConfirmedAt:   FormatTime(*reservation.ConfirmedAt),
	}
}

// FormatTime renders a timestamp as RFC 3339 in UTC with second precision.
// Fractions are dropped, never rounded up, so a client never believes a
// deadline is later than it is.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
