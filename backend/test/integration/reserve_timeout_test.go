package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/route"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
)

// AC-OP-03: the whole chain, from a query stuck in the database to the
// response. Another transaction holds the item row, so the reserve cannot
// finish and must run into its own deadline.
func TestReserve_TimesOutWhileItemIsLocked(t *testing.T) {
	ctx := context.Background()
	itemID := newItem(t, 5)

	// The same app as every other test, with a short request timeout.
	engine := infrastructure.NewGin()
	route.RegisterRoute(engine, handler.NewInventoryHandler(inventoryUsecase), handler.NewHealthHandler(db),
		50*time.Millisecond)

	blocker, err := db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(ctx) }()
	_, err = blocker.Exec(ctx, `SELECT 1 FROM inv_item WHERE item_id = $1 FOR UPDATE`, itemID)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, reserveTarget,
		strings.NewReader(reserveBody("usr_1", itemID, 1))))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, "body = %s", rec.Body)
	require.Equal(t, exception.ErrServiceUnavailable.Code, decodeError(t, rec).Error.Code)
	require.NotEmpty(t, rec.Header().Get("Retry-After"), "Retry-After header")

	// The cancelled statement is gone from the database before the lock is
	// released, so it cannot complete afterwards.
	require.Eventually(t, func() bool {
		var waiting int
		err := db.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query LIKE '%' || $1 || '%'`, "inv_reservation",
		).Scan(&waiting)
		return err == nil && waiting == 0
	}, 5*time.Second, 5*time.Millisecond, "the timed-out statement is still waiting for the lock")
	require.NoError(t, blocker.Rollback(ctx))

	// Nothing was reserved, and the item works normally once it is free.
	_, reserved := stockOf(t, itemID)
	require.Zero(t, reserved, "reserved stock after a timed-out reserve")
	reserve(t, itemID, 1)
	_, reserved = stockOf(t, itemID)
	require.Equal(t, 1, reserved, "reserved stock after the lock was released")
}
