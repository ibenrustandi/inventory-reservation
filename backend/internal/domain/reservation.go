package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Quantity limits for one reservation.
const (
	MinQuantity = 1
	MaxQuantity = 1000
)

// ReservationStatus is the state of a reservation. A reservation starts
// PENDING and changes exactly once, to CONFIRMED or EXPIRED.
type ReservationStatus string

const (
	ReservationPending   ReservationStatus = "PENDING"
	ReservationConfirmed ReservationStatus = "CONFIRMED"
	ReservationExpired   ReservationStatus = "EXPIRED"
)

// Reservation is a hold of a quantity of one item for one user, with a
// deadline.
type Reservation struct {
	ID        string
	UserID    string
	ItemID    string
	Quantity  int
	Status    ReservationStatus
	ExpiresAt time.Time
}

// NewReservationID returns a new random reservation ID. IDs cannot be
// guessed from one another.
func NewReservationID() string {
	b := make([]byte, 16)
	// crypto/rand.Read never returns an error.
	_, _ = rand.Read(b)
	return "res_" + hex.EncodeToString(b)
}

// ValidQuantity reports whether q is an allowed reservation quantity.
func ValidQuantity(q int) bool {
	return q >= MinQuantity && q <= MaxQuantity
}
