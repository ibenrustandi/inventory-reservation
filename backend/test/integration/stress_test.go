package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests run many requests at once against the real router and a real
// PostgreSQL database, then check the stock invariants from SQL. They are
// meant to be run with the race detector.

// result is the outcome of one request made from a worker goroutine.
type result struct {
	status int
	body   []byte
}

// call sends a POST and returns its outcome. Unlike post, it never fails
// the test itself, so it is safe to use from worker goroutines.
func call(target, body string) result {
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
	return result{status: rec.Code, body: rec.Body.Bytes()}
}

func (r result) reservationID() string {
	var body struct {
		ReservationID string `json:"reservation_id"`
	}
	_ = json.Unmarshal(r.body, &body)
	return body.ReservationID
}

func (r result) confirmedAt() string {
	var body struct {
		ConfirmedAt string `json:"confirmed_at"`
	}
	_ = json.Unmarshal(r.body, &body)
	return body.ConfirmedAt
}

// inParallel runs work(i) for i in [0, n) on n goroutines that all start at
// the same moment, and waits for them to finish.
func inParallel(n int, work func(i int)) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			work(i)
		}()
	}
	close(start)
	wg.Wait()
}

// sweepUntil runs sweeps back to back, as several impatient sweepers would,
// until stop is closed. It returns a function that waits for it to end.
func sweepUntil(t *testing.T, stop <-chan struct{}) (wait func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				if _, err := inventoryUsecase.ReleaseExpired(context.Background()); err != nil {
					t.Errorf("sweep: %v", err)
					return
				}
			}
		}
	}()
	return func() { <-done }
}

// requireInvariants checks INV-1 to INV-5 of the specification for one item
// straight from the database.
func requireInvariants(t *testing.T, itemID string, initialStock int) {
	t.Helper()
	var (
		total, reserved, pendingQty, confirmedQty int
		badConfirmed, badExpired, badPending      int
	)
	err := db.QueryRow(context.Background(), `
		SELECT i.item_stock, i.item_stock_reserved,
		       COALESCE(SUM(r.reservation_quantity) FILTER (WHERE r.reservation_status = 'PENDING'), 0),
		       COALESCE(SUM(r.reservation_quantity) FILTER (WHERE r.reservation_status = 'CONFIRMED'), 0),
		       COUNT(*) FILTER (WHERE r.reservation_status = 'CONFIRMED' AND (
		           r.reservation_confirmed_date IS NULL
		           OR r.reservation_confirmed_date >= r.reservation_expires_date
		           OR r.reservation_expired_date IS NOT NULL)),
		       COUNT(*) FILTER (WHERE r.reservation_status = 'EXPIRED' AND (
		           r.reservation_expired_date IS NULL
		           OR r.reservation_expired_date < r.reservation_expires_date
		           OR r.reservation_confirmed_date IS NOT NULL)),
		       COUNT(*) FILTER (WHERE r.reservation_status = 'PENDING' AND (
		           r.reservation_confirmed_date IS NOT NULL
		           OR r.reservation_expired_date IS NOT NULL))
		FROM inv_item i
		LEFT JOIN inv_reservation r USING (item_id)
		WHERE i.item_id = $1
		GROUP BY i.item_id`, itemID,
	).Scan(&total, &reserved, &pendingQty, &confirmedQty, &badConfirmed, &badExpired, &badPending)
	require.NoError(t, err, "read invariants of %s", itemID)

	require.GreaterOrEqual(t, reserved, 0, "INV-1: reserved stock is negative")
	require.LessOrEqual(t, reserved, total, "INV-1: reserved stock exceeds physical stock (oversold)")
	require.Equal(t, pendingQty, reserved, "INV-2: reserved stock does not match the pending reservations")
	require.Equal(t, initialStock, total+confirmedQty, "INV-3: physical stock plus confirmed quantity is not the initial stock")
	require.Zero(t, badConfirmed, "INV-4: a reservation was confirmed at or after its deadline, or confirmed and expired")
	require.Zero(t, badExpired, "INV-5: a reservation was released before its deadline, or released and confirmed")
	require.Zero(t, badPending, "a pending reservation carries a confirmed or expired date")
}

// AC-CC-01
func TestStress_NoOversell(t *testing.T) {
	const stock, buyers = 100, 1000
	itemID := newItem(t, stock)
	var created, conflicts, other atomic.Int32

	inParallel(buyers, func(i int) {
		switch call(reserveTarget, reserveBody(fmt.Sprintf("usr_%d", i), itemID, 1)).status {
		case http.StatusCreated:
			created.Add(1)
		case http.StatusConflict:
			conflicts.Add(1)
		default:
			other.Add(1)
		}
	})

	require.Zero(t, other.Load(), "responses that were neither 201 nor 409")
	require.EqualValues(t, stock, created.Load(), "successful reservations")
	require.EqualValues(t, buyers-stock, conflicts.Load(), "INSUFFICIENT_STOCK responses")
	total, reserved := stockOf(t, itemID)
	require.Equal(t, stock, total)
	require.Equal(t, stock, reserved)
	requireInvariants(t, itemID, stock)
}

// AC-CC-02
func TestStress_MixedQuantities(t *testing.T) {
	const stock, buyers = 200, 500
	itemID := newItem(t, stock)
	quantityOf := func(i int) int { return 1 + i%5 }
	statuses := make([]int, buyers)

	inParallel(buyers, func(i int) {
		statuses[i] = call(reserveTarget, reserveBody(fmt.Sprintf("usr_%d", i), itemID, quantityOf(i))).status
	})

	reservedByResponses := 0
	for i, status := range statuses {
		switch status {
		case http.StatusCreated:
			reservedByResponses += quantityOf(i)
		case http.StatusConflict:
		default:
			t.Fatalf("buyer %d got status %d", i, status)
		}
	}
	total, reserved := stockOf(t, itemID)
	require.Equal(t, stock, total)
	require.LessOrEqual(t, reserved, stock, "oversold")
	require.Equal(t, reservedByResponses, reserved, "reserved stock does not match what the 201 responses promised")
	// Stock only went down during this test, so every refused request must
	// have asked for more than is left even now.
	available := total - reserved
	for i, status := range statuses {
		if status == http.StatusConflict {
			require.Greater(t, quantityOf(i), available,
				"buyer %d was refused %d units although %d are still available", i, quantityOf(i), available)
		}
	}
	requireInvariants(t, itemID, stock)
}

// AC-CC-03
func TestStress_DoubleConfirm(t *testing.T) {
	const stock, quantity, attempts = 10, 3, 50
	itemID := newItem(t, stock)
	reservationID := reserve(t, itemID, quantity)
	body := fmt.Sprintf(`{"reservation_id":%q}`, reservationID)
	results := make([]result, attempts)

	inParallel(attempts, func(i int) { results[i] = call(confirmTarget, body) })

	confirmedAt := results[0].confirmedAt()
	require.NotEmpty(t, confirmedAt)
	for i, r := range results {
		require.Equal(t, http.StatusOK, r.status, "attempt %d: body = %s", i, r.body)
		require.Equal(t, confirmedAt, r.confirmedAt(), "attempt %d reported a different confirmation time", i)
	}
	total, reserved := stockOf(t, itemID)
	require.Equal(t, stock-quantity, total, "physical stock was not decremented exactly once")
	require.Zero(t, reserved)
	requireInvariants(t, itemID, stock)
}

// AC-CC-04
func TestStress_ConfirmVsSweeper(t *testing.T) {
	const reservations = 200
	itemID := newItem(t, reservations)
	ids := make([]string, reservations)
	for i := range ids {
		ids[i] = reserve(t, itemID, 1)
	}
	// Odd reservations pass their deadline before anyone tries to confirm.
	for i, id := range ids {
		if i%2 == 1 {
			makeDue(t, id)
		}
	}
	statuses := make([]int, reservations)

	stop := make(chan struct{})
	waitForSweeper := sweepUntil(t, stop)
	inParallel(reservations, func(i int) {
		statuses[i] = call(confirmTarget, fmt.Sprintf(`{"reservation_id":%q}`, ids[i])).status
	})
	close(stop)
	waitForSweeper()
	sweepOnce(t) // release anything still due

	for i, id := range ids {
		if i%2 == 0 {
			require.Equal(t, http.StatusOK, statuses[i], "reservation %d before its deadline", i)
			require.Equal(t, "CONFIRMED", statusOf(t, id), "reservation %d", i)
		} else {
			require.Equal(t, http.StatusGone, statuses[i], "reservation %d past its deadline", i)
			require.Equal(t, "EXPIRED", statusOf(t, id), "reservation %d", i)
		}
	}
	total, reserved := stockOf(t, itemID)
	require.Equal(t, reservations/2, total, "physical stock: half were confirmed")
	require.Zero(t, reserved, "reserved stock: every hold was confirmed or released")
	requireInvariants(t, itemID, reservations)
}

// AC-CC-05
func TestStress_InlineReleaseVsSweeper(t *testing.T) {
	const stock, buyers = 50, 300
	itemID := newItem(t, stock)
	overdue := make([]string, stock)
	for i := range overdue {
		overdue[i] = reserve(t, itemID, 1)
	}
	// Every unit is held by a reservation that has just passed its deadline.
	for _, id := range overdue {
		makeDue(t, id)
	}
	var created, conflicts, other atomic.Int32

	stop := make(chan struct{})
	waitForSweeper := sweepUntil(t, stop)
	inParallel(buyers, func(i int) {
		switch call(reserveTarget, reserveBody(fmt.Sprintf("usr_%d", i), itemID, 1)).status {
		case http.StatusCreated:
			created.Add(1)
		case http.StatusConflict:
			conflicts.Add(1)
		default:
			other.Add(1)
		}
	})
	close(stop)
	waitForSweeper()
	sweepOnce(t)

	require.Zero(t, other.Load(), "responses that were neither 201 nor 409")
	require.LessOrEqual(t, int(created.Load()), stock, "oversold")
	require.Positive(t, created.Load(), "no buyer got stock that overdue holds were sitting on")
	// Each overdue hold was released exactly once, by the sweeper or by a
	// reserve: the counter equals the new holds, no more and no less.
	for _, id := range overdue {
		require.Equal(t, "EXPIRED", statusOf(t, id))
	}
	total, reserved := stockOf(t, itemID)
	require.Equal(t, stock, total)
	require.EqualValues(t, created.Load(), reserved, "reserved stock does not match the new reservations")
	requireInvariants(t, itemID, stock)
}

// AC-CC-06
func TestStress_FullChurn(t *testing.T) {
	const stock, workers = 40, 360
	items := []string{newItem(t, stock), newItem(t, stock), newItem(t, stock)}
	var unexpected atomic.Int32
	expect := func(what string, got result, want int) {
		if got.status != want {
			unexpected.Add(1)
			t.Errorf("%s: status = %d, want %d; body = %s", what, got.status, want, got.body)
		}
	}

	stop := make(chan struct{})
	waitForSweeper := sweepUntil(t, stop)
	inParallel(workers, func(i int) {
		itemID := items[i%len(items)]
		reserved := call(reserveTarget, reserveBody(fmt.Sprintf("usr_%d", i), itemID, 1+i%3))
		if reserved.status == http.StatusConflict {
			return
		}
		expect("reserve", reserved, http.StatusCreated)
		id := reserved.reservationID()
		confirmBody := fmt.Sprintf(`{"reservation_id":%q}`, id)

		switch (i / len(items)) % 4 {
		case 0: // buy it
			expect("confirm", call(confirmTarget, confirmBody), http.StatusOK)
			expect("confirm again", call(confirmTarget, confirmBody), http.StatusOK)
		case 1: // let it lapse, then try to buy it
			if _, err := db.Exec(context.Background(), `
				UPDATE inv_reservation SET reservation_expires_date = now() - interval '1 second'
				WHERE reservation_id = $1`, id); err != nil {
				t.Errorf("make due: %v", err)
				return
			}
			expect("confirm after deadline", call(confirmTarget, confirmBody), http.StatusGone)
		case 2: // let it lapse and walk away
			if _, err := db.Exec(context.Background(), `
				UPDATE inv_reservation SET reservation_expires_date = now() - interval '1 second'
				WHERE reservation_id = $1`, id); err != nil {
				t.Errorf("make due: %v", err)
			}
		case 3: // keep holding it
		}
	})
	close(stop)
	waitForSweeper()
	sweepOnce(t)

	require.Zero(t, unexpected.Load(), "unexpected responses")
	for _, itemID := range items {
		requireInvariants(t, itemID, stock)
		// After the final sweep nothing overdue is still holding stock.
		var stillDue int
		require.NoError(t, db.QueryRow(context.Background(), `
			SELECT count(*) FROM inv_reservation
			WHERE item_id = $1 AND reservation_status = 'PENDING' AND reservation_expires_date <= now()`,
			itemID).Scan(&stillDue))
		require.Zero(t, stillDue, "overdue reservations still pending on %s", itemID)
	}
}
