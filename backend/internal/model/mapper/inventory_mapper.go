// Package mapper converts domain types into API response structures.
package mapper

import (
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
