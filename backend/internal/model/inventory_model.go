package model

// GetStockRequest asks for the stock breakdown of one item.
type GetStockRequest struct {
	ItemID string
}

// StockResponse is the stock breakdown of one item.
type StockResponse struct {
	ItemID         string `json:"item_id"`
	TotalStock     int    `json:"total_stock"`
	ReservedStock  int    `json:"reserved_stock"`
	AvailableStock int    `json:"available_stock"`
}
