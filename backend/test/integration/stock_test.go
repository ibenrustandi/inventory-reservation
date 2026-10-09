package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// AC-STK-01
func TestStock_Success(t *testing.T) {
	itemID := newItem(t, 100)
	// Set the counter directly; reservations are not what this test is about.
	if _, err := db.Exec(context.Background(),
		`UPDATE inv_item SET item_stock_reserved = 15 WHERE item_id = $1`, itemID,
	); err != nil {
		t.Fatalf("set reserved stock: %v", err)
	}

	rec := get(t, "/api/v1/inventory/stock?item_id="+itemID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := map[string]any{
		"item_id":         itemID,
		"total_stock":     float64(100),
		"reserved_stock":  float64(15),
		"available_stock": float64(85),
	}
	if len(got) != len(want) {
		t.Fatalf("body = %v, want exactly %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

// AC-STK-02
func TestStock_ItemNotFound(t *testing.T) {
	rec := get(t, "/api/v1/inventory/stock?item_id=item_does_not_exist")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNotFound, rec.Body)
	}
	var body struct {
		Status string `json:"status"`
		Error  struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "error" || body.Error.Code != "ITEM_NOT_FOUND" || body.RequestID == "" {
		t.Fatalf("body = %s, want error envelope with code ITEM_NOT_FOUND", rec.Body)
	}
	if body.Error.Details["item_id"] != "item_does_not_exist" {
		t.Fatalf("details = %v, want the requested item_id", body.Error.Details)
	}
}
