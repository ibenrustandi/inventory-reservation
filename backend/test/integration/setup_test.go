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
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

const postgresImage = "postgres:17-alpine"

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
	inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository)
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
