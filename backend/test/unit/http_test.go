package unit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/middleware"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

func TestHealth(t *testing.T) {
	rec := do(t, newApp(noUsecase, &mocks.Pinger{}), http.MethodGet, "/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), `{"status":"ok"}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "database answers", err: nil, want: http.StatusOK},
		{name: "database down", err: errors.New("connection refused"), want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newApp(noUsecase, &mocks.Pinger{Err: tt.err}), http.MethodGet, "/ready")
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestRouter_UnknownRouteAndMethod(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantCode   string
	}{
		{"unknown route", http.MethodGet, "/nope", http.StatusNotFound, exception.ErrRouteNotFound.Code},
		{"wrong method", http.MethodPost, "/api/v1/inventory/stock", http.StatusMethodNotAllowed, exception.ErrMethodNotAllowed.Code},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newApp(noUsecase, &mocks.Pinger{}), tt.method, tt.target)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decodeError(t, rec).Error.Code; got != tt.wantCode {
				t.Fatalf("code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestRequestID_ReusesValidClientID(t *testing.T) {
	tests := []struct {
		name   string
		sent   string
		reused bool
	}{
		{"valid", "client-id_123", true},
		{"invalid characters", "bad id!", false},
		{"absent", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			if tt.sent != "" {
				req.Header.Set(middleware.RequestIDHeader, tt.sent)
			}
			rec := httptest.NewRecorder()
			newApp(noUsecase, &mocks.Pinger{}).ServeHTTP(rec, req)

			got := rec.Header().Get(middleware.RequestIDHeader)
			if got == "" {
				t.Fatal("response has no request ID")
			}
			if reused := got == tt.sent; reused != tt.reused {
				t.Fatalf("response ID = %q, sent %q, reused = %v, want %v", got, tt.sent, reused, tt.reused)
			}
		})
	}
}
