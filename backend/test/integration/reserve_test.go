package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

const reserveTarget = "/api/v1/inventory/reserve"

func reserveBody(userID, itemID string, quantity int) string {
	return fmt.Sprintf(`{"user_id":%q,"item_id":%q,"quantity":%d}`, userID, itemID, quantity)
}

// AC-RES-01
func TestReserve_Success(t *testing.T) {
	itemID := newItem(t, 100)

	rec := post(t, reserveTarget, reserveBody("usr_9981", itemID, 2))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	var got struct {
		Status        string `json:"status"`
		ReservationID string `json:"reservation_id"`
		ItemID        string `json:"item_id"`
		Quantity      int    `json:"quantity"`
		ExpiresAt     string `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Status != "success" || got.ItemID != itemID || got.Quantity != 2 {
		t.Fatalf("body = %s, want status success, item %s, quantity 2", rec.Body, itemID)
	}
	if !strings.HasPrefix(got.ReservationID, "res_") {
		t.Fatalf("reservation_id = %q, want res_ prefix", got.ReservationID)
	}
	if !strings.HasSuffix(got.ExpiresAt, "Z") {
		t.Fatalf("expires_at = %q, want UTC", got.ExpiresAt)
	}
	expiresAt, err := time.Parse(time.RFC3339, got.ExpiresAt)
	if err != nil {
		t.Fatalf("expires_at = %q is not RFC 3339: %v", got.ExpiresAt, err)
	}

	// The stored reservation matches the response and lasts exactly the
	// configured lifetime, measured by the database's own clock.
	var (
		userID, status string
		quantity       int
		lifetime       time.Duration
		storedExpiry   time.Time
	)
	err = db.QueryRow(context.Background(), `
		SELECT user_id, reservation_status, reservation_quantity,
		       reservation_expires_date - reservation_date, reservation_expires_date
		FROM inv_reservation WHERE reservation_id = $1 AND item_id = $2`,
		got.ReservationID, itemID,
	).Scan(&userID, &status, &quantity, &lifetime, &storedExpiry)
	if err != nil {
		t.Fatalf("read reservation: %v", err)
	}
	if userID != "usr_9981" || status != "PENDING" || quantity != 2 {
		t.Fatalf("stored reservation = user %s status %s quantity %d", userID, status, quantity)
	}
	if lifetime != reservationTTL {
		t.Fatalf("reservation lifetime = %s, want %s", lifetime, reservationTTL)
	}
	if !expiresAt.Equal(storedExpiry.Truncate(time.Second)) {
		t.Fatalf("expires_at = %s, stored %s", expiresAt, storedExpiry)
	}

	if total, reserved := stockOf(t, itemID); total != 100 || reserved != 2 {
		t.Fatalf("stock = total %d reserved %d, want 100 and 2", total, reserved)
	}
}

// AC-RES-02
func TestReserve_InsufficientStock(t *testing.T) {
	itemID := newItem(t, 1)

	rec := post(t, reserveTarget, reserveBody("usr_1", itemID, 2))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusConflict, rec.Body)
	}
	body := decodeError(t, rec)
	if body.Error.Code != "INSUFFICIENT_STOCK" {
		t.Fatalf("code = %q, want INSUFFICIENT_STOCK", body.Error.Code)
	}
	if body.Error.Details["available_stock"] != float64(1) || body.Error.Details["requested"] != float64(2) {
		t.Fatalf("details = %v, want available_stock 1 and requested 2", body.Error.Details)
	}

	// Nothing changed.
	if total, reserved := stockOf(t, itemID); total != 1 || reserved != 0 {
		t.Fatalf("stock = total %d reserved %d, want 1 and 0", total, reserved)
	}
	var reservations int
	if err := db.QueryRow(context.Background(),
		`SELECT count(*) FROM inv_reservation WHERE item_id = $1`, itemID,
	).Scan(&reservations); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if reservations != 0 {
		t.Fatalf("reservations = %d, want 0", reservations)
	}
}

// AC-RES-01, at the boundary: the last units can be reserved, and then no more.
func TestReserve_ExactlyAllStock(t *testing.T) {
	itemID := newItem(t, 3)

	if rec := post(t, reserveTarget, reserveBody("usr_1", itemID, 3)); rec.Code != http.StatusCreated {
		t.Fatalf("reserve all: status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	rec := post(t, reserveTarget, reserveBody("usr_2", itemID, 1))
	if rec.Code != http.StatusConflict {
		t.Fatalf("reserve one more: status = %d, want %d; body = %s", rec.Code, http.StatusConflict, rec.Body)
	}
	if total, reserved := stockOf(t, itemID); total != 3 || reserved != 3 {
		t.Fatalf("stock = total %d reserved %d, want 3 and 3", total, reserved)
	}
}

// AC-RES-03
func TestReserve_ItemNotFound(t *testing.T) {
	rec := post(t, reserveTarget, reserveBody("usr_1", "item_does_not_exist", 1))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body)
	}
	if code := decodeError(t, rec).Error.Code; code != "ITEM_NOT_FOUND" {
		t.Fatalf("code = %q, want ITEM_NOT_FOUND", code)
	}
}
