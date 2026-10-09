// Package domain holds the core types and rules of the inventory system.
// It imports nothing else from this project.
package domain

// Stock is the stock breakdown of one item.
type Stock struct {
	ItemID string
	// Total is the number of physical units on hand, including reserved ones.
	Total int
	// Reserved is the number of units held by pending reservations.
	Reserved int
}

// Available is the number of units that can still be reserved.
func (s Stock) Available() int {
	return s.Total - s.Reserved
}
