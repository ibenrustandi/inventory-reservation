// Package repository is the data access layer.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
)

// InventoryRepository is the data access the inventory use cases need.
type InventoryRepository interface {
	// FindStockByItemID returns the stock of an item, or
	// exception.ErrItemNotFound.
	FindStockByItemID(ctx context.Context, itemID string) (*domain.Stock, error)
	// Reserve holds reservation.Quantity units of the item and stores the
	// reservation, atomically, if enough stock is available. It reports
	// whether the reservation was made and, if so, sets its Status and
	// ExpiresAt. It returns false both when stock is insufficient and when
	// the item does not exist.
	Reserve(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error)
	// Confirm turns a pending reservation that is still before its deadline
	// into a purchase, atomically: the reservation becomes CONFIRMED and
	// the item's physical and reserved stock both drop by its quantity.
	// It reports whether it confirmed the reservation; false means the
	// reservation is missing, already confirmed, or past its deadline.
	Confirm(ctx context.Context, reservationID string) (*domain.Reservation, bool, error)
	// FindReservationByID returns a reservation, or
	// exception.ErrReservationNotFound.
	FindReservationByID(ctx context.Context, reservationID string) (*domain.Reservation, error)
}

// InventoryRepositoryImpl is the PostgreSQL implementation.
type InventoryRepositoryImpl struct {
	DB *pgxpool.Pool
}

// NewInventoryRepository returns a repository that runs its queries on the
// given pool.
func NewInventoryRepository(db *pgxpool.Pool) InventoryRepository {
	return &InventoryRepositoryImpl{DB: db}
}

func (r *InventoryRepositoryImpl) FindStockByItemID(ctx context.Context, itemID string) (*domain.Stock, error) {
	stock := &domain.Stock{ItemID: itemID}
	err := r.DB.QueryRow(ctx,
		`SELECT item_stock, item_stock_reserved FROM inv_item WHERE item_id = $1`,
		itemID,
	).Scan(&stock.Total, &stock.Reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, exception.ErrItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find stock by item id: %w", err)
	}
	return stock, nil
}

// reserveSQL is the whole oversell protection. The UPDATE takes the item's
// row lock and re-checks availability against the latest committed row, so
// two concurrent requests can never both take the last units. The INSERT
// runs only if the UPDATE changed a row, in the same statement.
const reserveSQL = `
WITH upd AS (
  UPDATE inv_item
  SET item_stock_reserved = item_stock_reserved + $3, updated_at = now()
  WHERE item_id = $2 AND item_stock - item_stock_reserved >= $3
  RETURNING item_id
)
INSERT INTO inv_reservation (reservation_id, user_id, item_id,
                             reservation_quantity, reservation_status, reservation_expires_date)
SELECT $1, $4, item_id, $3, 'PENDING', now() + $5::interval FROM upd
RETURNING reservation_expires_date`

func (r *InventoryRepositoryImpl) Reserve(ctx context.Context, reservation *domain.Reservation, ttl time.Duration) (bool, error) {
	var expiresAt time.Time
	err := r.DB.QueryRow(ctx, reserveSQL,
		reservation.ID, reservation.ItemID, reservation.Quantity, reservation.UserID, ttl,
	).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reserve: %w", err)
	}
	reservation.Status = domain.ReservationPending
	reservation.ExpiresAt = expiresAt
	return true, nil
}

// The deadline is checked here, by the database clock, so a reservation
// past its deadline can never be confirmed even if its stock has not been
// released yet.
const confirmReservationSQL = `
UPDATE inv_reservation
SET reservation_status = 'CONFIRMED', reservation_confirmed_date = now()
WHERE reservation_id = $1 AND reservation_status = 'PENDING' AND reservation_expires_date > now()
RETURNING user_id, item_id, reservation_quantity, reservation_expires_date, reservation_confirmed_date`

const confirmItemSQL = `
UPDATE inv_item
SET item_stock = item_stock - $2, item_stock_reserved = item_stock_reserved - $2, updated_at = now()
WHERE item_id = $1`

func (r *InventoryRepositoryImpl) Confirm(ctx context.Context, reservationID string) (*domain.Reservation, bool, error) {
	var reservation *domain.Reservation
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		reservation = nil
		confirmed := &domain.Reservation{ID: reservationID, Status: domain.ReservationConfirmed}
		var confirmedAt time.Time

		// Lock order: reservation row first, then item row.
		err := tx.QueryRow(ctx, confirmReservationSQL, reservationID).Scan(
			&confirmed.UserID, &confirmed.ItemID, &confirmed.Quantity, &confirmed.ExpiresAt, &confirmedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		confirmed.ConfirmedAt = &confirmedAt

		tag, err := tx.Exec(ctx, confirmItemSQL, confirmed.ItemID, confirmed.Quantity)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("item %s of reservation %s not found", confirmed.ItemID, reservationID)
		}
		reservation = confirmed
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("confirm: %w", err)
	}
	return reservation, reservation != nil, nil
}

func (r *InventoryRepositoryImpl) FindReservationByID(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	reservation := &domain.Reservation{ID: reservationID}
	err := r.DB.QueryRow(ctx, `
		SELECT user_id, item_id, reservation_quantity, reservation_status,
		       reservation_expires_date, reservation_confirmed_date
		FROM inv_reservation WHERE reservation_id = $1`,
		reservationID,
	).Scan(&reservation.UserID, &reservation.ItemID, &reservation.Quantity, &reservation.Status,
		&reservation.ExpiresAt, &reservation.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, exception.ErrReservationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find reservation by id: %w", err)
	}
	return reservation, nil
}

// PostgreSQL error codes for a transaction the database aborted to resolve
// a conflict. Running it again is safe and expected to succeed.
const (
	codeSerializationFailure = "40001"
	codeDeadlockDetected     = "40P01"
)

// inTx runs fn in a transaction. If the database aborts it because of a
// deadlock or serialization failure, it is run once more.
func (r *InventoryRepositoryImpl) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	err := pgx.BeginFunc(ctx, r.DB, fn)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == codeDeadlockDetected || pgErr.Code == codeSerializationFailure) {
		err = pgx.BeginFunc(ctx, r.DB, fn)
	}
	return err
}
