package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// AC-EXP-01: no sweeper runs in this test; the reserve itself must free the
// stock held by the overdue reservation.
func TestReserve_ReleasesDueHoldsInline(t *testing.T) {
	itemID := newItem(t, 2)
	overdue := reserve(t, itemID, 2)
	makeDue(t, overdue)

	rec := post(t, reserveTarget, reserveBody("usr_2", itemID, 2))

	require.Equal(t, http.StatusCreated, rec.Code, "body = %s", rec.Body)
	require.Equal(t, "EXPIRED", statusOf(t, overdue), "overdue reservation")
	require.NotNil(t, expiredDateOf(t, overdue), "released reservation has no expired date")
	total, reserved := stockOf(t, itemID)
	require.Equal(t, 2, total, "physical stock")
	require.Equal(t, 2, reserved, "reserved stock: only the new hold")

	// The stock was released exactly once: a sweep now finds nothing for
	// this item, and the counter still matches the one pending hold.
	sweepOnce(t)
	_, reserved = stockOf(t, itemID)
	require.Equal(t, 2, reserved, "reserved stock after a sweep")
}

// AC-EXP-02
func TestReserve_DoesNotReleaseActiveHolds(t *testing.T) {
	itemID := newItem(t, 2)
	active := reserve(t, itemID, 2)

	rec := post(t, reserveTarget, reserveBody("usr_2", itemID, 2))

	require.Equal(t, http.StatusConflict, rec.Code, "body = %s", rec.Body)
	require.Equal(t, "INSUFFICIENT_STOCK", decodeError(t, rec).Error.Code)
	require.Equal(t, "PENDING", statusOf(t, active), "a reservation before its deadline was released")
	total, reserved := stockOf(t, itemID)
	require.Equal(t, 2, total, "physical stock")
	require.Equal(t, 2, reserved, "reserved stock")
}

// Releasing overdue holds may still not free enough: the answer is then
// INSUFFICIENT_STOCK with the availability after the release.
func TestReserve_InlineReleaseFreesTooLittle(t *testing.T) {
	itemID := newItem(t, 5)
	overdue := reserve(t, itemID, 1)
	active := reserve(t, itemID, 4)
	makeDue(t, overdue)

	rec := post(t, reserveTarget, reserveBody("usr_2", itemID, 3))

	require.Equal(t, http.StatusConflict, rec.Code, "body = %s", rec.Body)
	body := decodeError(t, rec)
	require.Equal(t, "INSUFFICIENT_STOCK", body.Error.Code)
	require.Equal(t, float64(1), body.Error.Details["available_stock"], "availability after the release")
	require.Equal(t, "EXPIRED", statusOf(t, overdue))
	require.Equal(t, "PENDING", statusOf(t, active))
	_, reserved := stockOf(t, itemID)
	require.Equal(t, 4, reserved, "reserved stock: only the active hold")
}

// A reserve releases overdue holds of its own item only.
func TestReserve_InlineReleaseLeavesOtherItemsAlone(t *testing.T) {
	sweepOnce(t)
	itemID, otherItemID := newItem(t, 1), newItem(t, 1)
	overdue := reserve(t, itemID, 1)
	otherOverdue := reserve(t, otherItemID, 1)
	makeDue(t, overdue)
	makeDue(t, otherOverdue)

	rec := post(t, reserveTarget, reserveBody("usr_2", itemID, 1))

	require.Equal(t, http.StatusCreated, rec.Code, "body = %s", rec.Body)
	require.Equal(t, "EXPIRED", statusOf(t, overdue))
	require.Equal(t, "PENDING", statusOf(t, otherOverdue), "another item's hold was released by this reserve")
	_, otherReserved := stockOf(t, otherItemID)
	require.Equal(t, 1, otherReserved, "other item's reserved stock")
}

// When stock is available the overdue holds are left for the sweeper: the
// fast path does no extra work.
func TestReserve_FastPathDoesNotRelease(t *testing.T) {
	sweepOnce(t)
	itemID := newItem(t, 5)
	overdue := reserve(t, itemID, 1)
	makeDue(t, overdue)

	rec := post(t, reserveTarget, reserveBody("usr_2", itemID, 2))

	require.Equal(t, http.StatusCreated, rec.Code, "body = %s", rec.Body)
	require.Equal(t, "PENDING", statusOf(t, overdue), "the fast path released a hold")
}
