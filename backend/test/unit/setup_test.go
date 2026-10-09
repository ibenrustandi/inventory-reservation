// Package unit tests each layer in isolation, with test doubles in place of
// the layer below. It needs no database.
package unit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/middleware"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/route"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

// newApp returns the real router and middleware on top of the given use
// cases and database check.
func newApp(inventoryUsecase usecase.InventoryUsecase, db handler.Pinger) http.Handler {
	app := infrastructure.NewGin()
	route.RegisterRoute(app, handler.NewInventoryHandler(inventoryUsecase), handler.NewHealthHandler(db))
	return app
}

// do sends a request with no body to the app.
func do(t *testing.T, app http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// decodeError parses the error envelope and fails the test if the body is
// not one.
func decodeError(t *testing.T, rec *httptest.ResponseRecorder) model.ErrorResponse {
	t.Helper()
	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v; body = %s", err, rec.Body)
	}
	if body.Status != "error" {
		t.Fatalf("status field = %q, want %q; body = %s", body.Status, "error", rec.Body)
	}
	header := rec.Header().Get(middleware.RequestIDHeader)
	if body.RequestID == "" || body.RequestID != header {
		t.Fatalf("request_id = %q, header = %q; want equal and non-empty", body.RequestID, header)
	}
	return body
}

// noUsecase is for tests that never reach a use case.
var noUsecase = &mocks.InventoryUsecase{}
