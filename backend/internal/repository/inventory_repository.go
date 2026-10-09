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
	// SweepDue releases up to limit reservations that are past their
	// deadline, across all items, oldest first, and returns how many it
	// released. Only one sweep runs at a time across every instance of the
	// service; if another one is running, it releases nothing.
	SweepDue(ctx context.Context, limit int) (int, error)
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

// releaseDueSQL releases reservations that are past their deadline: it marks
// them EXPIRED and gives their quantity back to the item, in one statement.
// It is the only place expiry is implemented.
//
//	$1 item ID, or NULL for every item
//	$2 maximum number of reservations, or NULL for no limit
//
// SKIP LOCKED lets several callers run at once without ever releasing the
// same reservation twice. The item update joins on the released rows, so
// when nothing is due no item row is locked or written.
const releaseDueSQL = `
WITH due AS (
  SELECT reservation_id
  FROM inv_reservation
  WHERE ($1::text IS NULL OR item_id = $1)
    AND reservation_status = 'PENDING'
    AND reservation_expires_date <= now()
  ORDER BY reservation_expires_date
  LIMIT $2::int
  FOR UPDATE SKIP LOCKED
), expired AS (
  UPDATE inv_reservation r
  SET reservation_status = 'EXPIRED', reservation_expired_date = now()
  FROM due
  WHERE r.reservation_id = due.reservation_id
  RETURNING r.item_id, r.reservation_quantity
), agg AS (
  SELECT item_id, SUM(reservation_quantity) AS qty, COUNT(*) AS n
  FROM expired
  GROUP BY item_id
), upd AS (
  UPDATE inv_item i
  SET item_stock_reserved = i.item_stock_reserved - agg.qty, updated_at = now()
  FROM agg
  WHERE i.item_id = agg.item_id
)
SELECT COALESCE(SUM(n), 0)::int FROM agg`

// querier is what releaseDue needs to run its statement: a pool or a
// transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// releaseDue runs releaseDueSQL and returns the number of reservations
// released. A nil itemID means every item; a nil limit means no limit.
func releaseDue(ctx context.Context, q querier, itemID *string, limit *int) (int, error) {
	var released int
	if err := q.QueryRow(ctx, releaseDueSQL, itemID, limit).Scan(&released); err != nil {
		return 0, err
	}
	return released, nil
}

// SweeperLockName names the advisory lock that lets only one sweep run at a
// time. Two sweeps at once would each lock several item rows in different
// orders and could deadlock.
const SweeperLockName = "inv_reservation_sweeper"

func (r *InventoryRepositoryImpl) SweepDue(ctx context.Context, limit int) (int, error) {
	var released int
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		released = 0
		// The lock is released automatically when the transaction ends.
		var locked bool
		if err := tx.QueryRow(ctx,
			`SELECT pg_try_advisory_xact_lock(hashtext($1))`, SweeperLockName,
		).Scan(&locked); err != nil {
			return err
		}
		if !locked {
			return nil
		}
		n, err := releaseDue(ctx, tx, nil, &limit)
		released = n
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("sweep due: %w", err)
	}
	return released, nil
}
