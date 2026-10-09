package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/repository"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/sweeper"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
)

// expiredDateOf reads when a reservation was released, or nil.
func expiredDateOf(t *testing.T, reservationID string) *time.Time {
	t.Helper()
	var expired *time.Time
	require.NoError(t, db.QueryRow(context.Background(),
		`SELECT reservation_expired_date FROM inv_reservation WHERE reservation_id = $1`, reservationID,
	).Scan(&expired))
	return expired
}

// AC-EXP-03
func TestSweepOnce_ReturnsStock(t *testing.T) {
	sweepOnce(t) // clear anything earlier tests left due
	itemID := newItem(t, 10)
	due := reserve(t, itemID, 2)
	active := reserve(t, itemID, 3)
	makeDue(t, due)

	released := sweepOnce(t)

	require.Equal(t, 1, released, "reservations released")
	require.Equal(t, "EXPIRED", statusOf(t, due))
	require.NotNil(t, expiredDateOf(t, due), "released reservation has no expired date")
	require.Equal(t, "PENDING", statusOf(t, active), "a reservation before its deadline was touched")
	total, reserved := stockOf(t, itemID)
	require.Equal(t, 10, total, "physical stock")
	require.Equal(t, 3, reserved, "reserved stock: only the active hold remains")

	// A second sweep finds nothing: stock is released exactly once.
	require.Equal(t, 0, sweepOnce(t), "second sweep")
	_, reserved = stockOf(t, itemID)
	require.Equal(t, 3, reserved, "reserved stock after second sweep")
}

func TestSweepOnce_ReleasesAcrossItems(t *testing.T) {
	sweepOnce(t)
	itemA, itemB := newItem(t, 10), newItem(t, 10)
	a1, a2 := reserve(t, itemA, 1), reserve(t, itemA, 4)
	b1 := reserve(t, itemB, 5)
	for _, id := range []string{a1, a2, b1} {
		makeDue(t, id)
	}

	released := sweepOnce(t)

	require.Equal(t, 3, released)
	_, reservedA := stockOf(t, itemA)
	_, reservedB := stockOf(t, itemB)
	require.Equal(t, 0, reservedA, "item A reserved stock")
	require.Equal(t, 0, reservedB, "item B reserved stock")
}

func TestSweepOnce_DrainsBacklogInBatches(t *testing.T) {
	sweepOnce(t)
	itemID := newItem(t, 10)
	for range 5 {
		makeDue(t, reserve(t, itemID, 1))
	}
	// Batches of 2 need three transactions to release 5 reservations.
	smallBatches := &usecase.InventoryUsecaseImpl{
		InventoryRepository: inventoryRepository,
		ReservationTTL:      reservationTTL,
		SweepBatchSize:      2,
	}

	released, err := sweeper.New(smallBatches, time.Hour).SweepOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 5, released)
	_, reserved := stockOf(t, itemID)
	require.Equal(t, 0, reserved)
}

// Only one sweep runs at a time across instances: a second one that cannot
// get the lock releases nothing and leaves the work to the first.
func TestSweepOnce_SkipsWhileAnotherSweepHoldsTheLock(t *testing.T) {
	sweepOnce(t)
	ctx := context.Background()
	itemID := newItem(t, 10)
	due := reserve(t, itemID, 2)
	makeDue(t, due)

	// Stand in for another instance that is in the middle of a sweep.
	other, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = other.Rollback(ctx) }()
	_, err = other.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, repository.SweeperLockName)
	require.NoError(t, err)

	require.Equal(t, 0, sweepOnce(t), "sweep while the lock is held")
	require.Equal(t, "PENDING", statusOf(t, due))

	require.NoError(t, other.Rollback(ctx))
	require.Equal(t, 1, sweepOnce(t), "sweep after the lock is free")
	require.Equal(t, "EXPIRED", statusOf(t, due))
}

// AC-EXP-04
func TestSweeperRun_ReleasesEventually(t *testing.T) {
	itemID := newItem(t, 10)
	reservationID := reserve(t, itemID, 4)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		sweeper.New(inventoryUsecase, 10*time.Millisecond).Run(ctx)
	}()
	defer func() {
		cancel()
		<-stopped
	}()

	// Still before its deadline: the running sweeper must leave it alone.
	require.Never(t, func() bool { return statusOf(t, reservationID) != "PENDING" },
		100*time.Millisecond, 10*time.Millisecond, "sweeper released a reservation before its deadline")

	makeDue(t, reservationID)

	// No request is made; the sweeper alone returns the stock.
	require.Eventually(t, func() bool { return statusOf(t, reservationID) == "EXPIRED" },
		5*time.Second, 10*time.Millisecond, "sweeper did not release the due reservation")
	_, reserved := stockOf(t, itemID)
	require.Equal(t, 0, reserved, "reserved stock")
}
