package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const confirmTarget = "/api/v1/inventory/confirm"

func confirm(t *testing.T, reservationID string) *httptest.ResponseRecorder {
	t.Helper()
	return post(t, confirmTarget, fmt.Sprintf(`{"reservation_id":%q}`, reservationID))
}

type confirmBody struct {
	Status        string `json:"status"`
	ReservationID string `json:"reservation_id"`
	ConfirmedAt   string `json:"confirmed_at"`
}

func decodeConfirm(t *testing.T, rec *httptest.ResponseRecorder) confirmBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	var body confirmBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

// AC-CON-01
func TestConfirm_Success(t *testing.T) {
	itemID := newItem(t, 100)
	reservationID := reserve(t, itemID, 2)

	got := decodeConfirm(t, confirm(t, reservationID))

	if got.Status != "success" || got.ReservationID != reservationID {
		t.Fatalf("body = %+v, want success for %s", got, reservationID)
	}
	if !strings.HasSuffix(got.ConfirmedAt, "Z") {
		t.Fatalf("confirmed_at = %q, want UTC", got.ConfirmedAt)
	}
	confirmedAt, err := time.Parse(time.RFC3339, got.ConfirmedAt)
	if err != nil {
		t.Fatalf("confirmed_at = %q is not RFC 3339: %v", got.ConfirmedAt, err)
	}
	var stored time.Time
	if err := db.QueryRow(context.Background(),
		`SELECT reservation_confirmed_date FROM inv_reservation WHERE reservation_id = $1`, reservationID,
	).Scan(&stored); err != nil {
		t.Fatalf("read confirmed date: %v", err)
	}
	if !confirmedAt.Equal(stored.Truncate(time.Second)) {
		t.Fatalf("confirmed_at = %s, stored %s", confirmedAt, stored)
	}
	if status := statusOf(t, reservationID); status != "CONFIRMED" {
		t.Fatalf("status = %s, want CONFIRMED", status)
	}

	// Physical and reserved stock both drop by 2, so available stays 98.
	total, reserved := stockOf(t, itemID)
	if total != 98 || reserved != 0 {
		t.Fatalf("stock = total %d reserved %d, want 98 and 0", total, reserved)
	}
}

// AC-CON-02
func TestConfirm_RepeatIsIdempotent(t *testing.T) {
	itemID := newItem(t, 10)
	reservationID := reserve(t, itemID, 3)
	first := decodeConfirm(t, confirm(t, reservationID))

	second := decodeConfirm(t, confirm(t, reservationID))

	if second != first {
		t.Fatalf("second confirm = %+v, want the same as the first %+v", second, first)
	}
	if total, reserved := stockOf(t, itemID); total != 7 || reserved != 0 {
		t.Fatalf("stock = total %d reserved %d, want 7 and 0 (decremented once)", total, reserved)
	}
}

// AC-CON-03
func TestConfirm_PastDeadlineNotYetReleased(t *testing.T) {
	itemID := newItem(t, 10)
	reservationID := reserve(t, itemID, 2)
	makeDue(t, reservationID)

	rec := confirm(t, reservationID)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusGone, rec.Body)
	}
	body := decodeError(t, rec)
	if body.Error.Code != "RESERVATION_EXPIRED" {
		t.Fatalf("code = %q, want RESERVATION_EXPIRED", body.Error.Code)
	}
	if _, err := time.Parse(time.RFC3339, fmt.Sprint(body.Error.Details["expires_at"])); err != nil {
		t.Fatalf("details = %v, want an RFC 3339 expires_at", body.Error.Details)
	}

	// The rejected confirm changed nothing: the hold is still there for the
	// expiry process to release.
	if status := statusOf(t, reservationID); status != "PENDING" {
		t.Fatalf("status = %s, want PENDING", status)
	}
	if total, reserved := stockOf(t, itemID); total != 10 || reserved != 2 {
		t.Fatalf("stock = total %d reserved %d, want 10 and 2", total, reserved)
	}
}

// AC-CON-04
func TestConfirm_Expired(t *testing.T) {
	itemID := newItem(t, 10)
	reservationID := reserve(t, itemID, 2)
	makeDue(t, reservationID)
	sweepOnce(t)

	rec := confirm(t, reservationID)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusGone, rec.Body)
	}
	if code := decodeError(t, rec).Error.Code; code != "RESERVATION_EXPIRED" {
		t.Fatalf("code = %q, want RESERVATION_EXPIRED", code)
	}
	if status := statusOf(t, reservationID); status != "EXPIRED" {
		t.Fatalf("status = %s, want EXPIRED", status)
	}
	if total, reserved := stockOf(t, itemID); total != 10 || reserved != 0 {
		t.Fatalf("stock = total %d reserved %d, want 10 and 0", total, reserved)
	}
}

// AC-CON-05
func TestConfirm_NotFound(t *testing.T) {
	rec := confirm(t, "res_does_not_exist")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body)
	}
	if code := decodeError(t, rec).Error.Code; code != "RESERVATION_NOT_FOUND" {
		t.Fatalf("code = %q, want RESERVATION_NOT_FOUND", code)
	}
}
