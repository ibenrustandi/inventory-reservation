package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
)

const checkViolation = "23514"

func TestMigrate_SeedsItem(t *testing.T) {
	ctx := context.Background()

	var stock, reserved int
	err := db.QueryRow(ctx,
		`SELECT item_stock, item_stock_reserved FROM inv_item WHERE item_id = 'item_4021'`,
	).Scan(&stock, &reserved)
	if err != nil {
		t.Fatalf("read seeded item: %v", err)
	}
	if stock != 100 || reserved != 0 {
		t.Fatalf("seeded item = stock %d reserved %d, want 100 and 0", stock, reserved)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {

	if err := infrastructure.Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestMigrate_UsesPrefixedBookkeepingTable(t *testing.T) {
	ctx := context.Background()

	for table, want := range map[string]bool{
		"inv_schema_migration": true,
		"goose_db_version":     false,
	} {
		var exists bool
		err := db.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if exists != want {
			t.Errorf("table %s exists = %v, want %v", table, exists, want)
		}
	}
}

func TestMigrate_CreatesExpiryIndexes(t *testing.T) {
	ctx := context.Background()

	for _, index := range []string{
		"inv_reservation_pending_expiry_idx",
		"inv_reservation_item_pending_expiry_idx",
	} {
		var exists bool
		err := db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'inv_reservation' AND indexname = $1)`,
			index,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("look up index %s: %v", index, err)
		}
		if !exists {
			t.Errorf("index %s does not exist", index)
		}
	}
}

func TestSchema_RejectsInvalidRows(t *testing.T) {
	ctx := context.Background()

	itemID := fmt.Sprintf("item_schema_%d", time.Now().UnixNano())
	if _, err := db.Exec(ctx,
		`INSERT INTO inv_item (item_id, item_stock) VALUES ($1, 5)`, itemID,
	); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	insertReservation := `INSERT INTO inv_reservation
		(reservation_id, user_id, item_id, reservation_quantity, reservation_status, reservation_expires_date)
		VALUES ($1, 'usr_1', $2, $3, $4, now() + interval '5 minutes')`

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "reserved above stock",
			sql:  `UPDATE inv_item SET item_stock_reserved = 6 WHERE item_id = $1`,
			args: []any{itemID},
		},
		{
			name: "negative reserved",
			sql:  `UPDATE inv_item SET item_stock_reserved = -1 WHERE item_id = $1`,
			args: []any{itemID},
		},
		{
			name: "negative stock",
			sql:  `UPDATE inv_item SET item_stock = -1 WHERE item_id = $1`,
			args: []any{itemID},
		},
		{
			name: "zero quantity",
			sql:  insertReservation,
			args: []any{itemID + "_res_a", itemID, 0, "PENDING"},
		},
		{
			name: "unknown status",
			sql:  insertReservation,
			args: []any{itemID + "_res_b", itemID, 1, "CANCELLED"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Exec(ctx, tt.sql, tt.args...)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != checkViolation {
				t.Fatalf("error = %v, want check violation (%s)", err, checkViolation)
			}
		})
	}
}
