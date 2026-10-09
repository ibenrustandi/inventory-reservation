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

// ReserveRequest asks to hold a quantity of an item for a user. Quantity is
// a pointer so a missing field can be told apart from zero.
type ReserveRequest struct {
	UserID   string `json:"user_id"`
	ItemID   string `json:"item_id"`
	Quantity *int   `json:"quantity"`
}

// ReserveResponse describes a reservation that was made.
type ReserveResponse struct {
	Status        string `json:"status"`
	ReservationID string `json:"reservation_id"`
	ItemID        string `json:"item_id"`
	Quantity      int    `json:"quantity"`
	// ExpiresAt is RFC 3339 in UTC.
	ExpiresAt string `json:"expires_at"`
}

// ConfirmRequest asks to turn a pending reservation into a purchase.
type ConfirmRequest struct {
	ReservationID string `json:"reservation_id"`
}

// ConfirmResponse describes a confirmed reservation.
type ConfirmResponse struct {
	Status        string `json:"status"`
	ReservationID string `json:"reservation_id"`
	// ConfirmedAt is RFC 3339 in UTC.
	ConfirmedAt string `json:"confirmed_at"`
}
