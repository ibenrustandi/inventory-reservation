// Package integration tests the service end to end against a real
// PostgreSQL database.
//
// By default it starts a throwaway PostgreSQL container for the test run.
// Set TEST_DATABASE_URL to use an already running database instead.
package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/route"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/repository"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
)

const (
	postgresImage = "postgres:17-alpine"
	// reservationTTL is the reservation lifetime the app under test uses.
	reservationTTL = 5 * time.Minute
)

var (
	// db is the migrated database shared by every test in this package.
	// Tests create their own rows to stay independent.
	db *pgxpool.Pool
	// app is the real router on top of the real use cases and repository.
	app http.Handler
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.DiscardHandler))

	cleanup, err := setup()
	if err != nil {
		// A missing database fails the run; these tests are never skipped.
		fmt.Fprintf(os.Stderr, "integration test setup: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func setup() (cleanup func(), err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stopContainer := func() {}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		container, err := tcpostgres.Run(ctx, postgresImage, tcpostgres.BasicWaitStrategies())
		if err != nil {
			return nil, fmt.Errorf("start postgres container: %w", err)
		}
		stopContainer = func() { _ = container.Terminate(context.Background()) }
		url, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			stopContainer()
			return nil, fmt.Errorf("container connection string: %w", err)
		}
	}

	db, err = infrastructure.NewPostgres(ctx, url)
	if err != nil {
		stopContainer()
		return nil, err
	}
	if err := infrastructure.Migrate(ctx, db); err != nil {
		db.Close()
		stopContainer()
		return nil, err
	}

	inventoryRepository := repository.NewInventoryRepository(db)
	inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)
	engine := infrastructure.NewGin()
	route.RegisterRoute(engine, handler.NewInventoryHandler(inventoryUsecase), handler.NewHealthHandler(db))
	app = engine

	return func() {
		db.Close()
		stopContainer()
	}, nil
}

// newItem inserts an item with the given stock and a unique ID, so each
// test works on its own row. It returns the item ID.
func newItem(t *testing.T, stock int) string {
	t.Helper()
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	itemID := "item_test_" + hex.EncodeToString(b)

	if _, err := db.Exec(context.Background(),
		`INSERT INTO inv_item (item_id, item_stock) VALUES ($1, $2)`, itemID, stock,
	); err != nil {
		t.Fatalf("insert test item: %v", err)
	}
	return itemID
}

// get sends a GET request to the app.
func get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// post sends a POST request with the given JSON body to the app.
func post(t *testing.T, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
	return rec
}

// stockOf reads an item's stock counters straight from the database.
func stockOf(t *testing.T, itemID string) (total, reserved int) {
	t.Helper()
	if err := db.QueryRow(context.Background(),
		`SELECT item_stock, item_stock_reserved FROM inv_item WHERE item_id = $1`, itemID,
	).Scan(&total, &reserved); err != nil {
		t.Fatalf("read stock of %s: %v", itemID, err)
	}
	return total, reserved
}

// errorBody is the error envelope as the tests read it.
type errorBody struct {
	Status string `json:"status"`
	Error  struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// decodeError parses the error envelope and fails the test if the body is
// not one.
func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v; body = %s", err, rec.Body)
	}
	if body.Status != "error" || body.RequestID == "" {
		t.Fatalf("body = %s, want an error envelope with a request_id", rec.Body)
	}
	return body
}
