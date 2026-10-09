package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/postgres"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/testdb"
)

const checkViolation = "23514"

func TestMigrate_SeedsItem(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()

	var stock, reserved int
	err := pool.QueryRow(ctx,
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
	pool := testdb.Pool(t)

	if err := postgres.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestMigrate_UsesPrefixedBookkeepingTable(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()

	for table, want := range map[string]bool{
		"inv_schema_migration": true,
		"goose_db_version":     false,
	} {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if exists != want {
			t.Errorf("table %s exists = %v, want %v", table, exists, want)
		}
	}
}

func TestSchema_RejectsInvalidRows(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()

	itemID := fmt.Sprintf("item_schema_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx,
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
			_, err := pool.Exec(ctx, tt.sql, tt.args...)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != checkViolation {
				t.Fatalf("error = %v, want check violation (%s)", err, checkViolation)
			}
		})
	}
}
