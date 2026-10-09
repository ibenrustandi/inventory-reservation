// Package testdb gives tests a migrated PostgreSQL database.
//
// By default it starts a throwaway PostgreSQL container, once per test
// binary. Set TEST_DATABASE_URL to use an already running database instead.
package testdb

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/postgres"
)

const image = "postgres:17-alpine"

var (
	once    sync.Once
	pool    *pgxpool.Pool
	initErr error
)

// Pool returns a connection pool to a migrated database shared by all tests
// in the binary. Tests must create their own rows to stay independent.
// A missing database fails the test; it is never skipped.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	once.Do(func() { pool, initErr = start() })
	if initErr != nil {
		t.Fatalf("test database: %v", initErr)
	}
	return pool
}

func start() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		// The container is removed by the testcontainers reaper when the
		// test binary exits.
		container, err := tcpostgres.Run(ctx, image, tcpostgres.BasicWaitStrategies())
		if err != nil {
			return nil, err
		}
		url, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return nil, err
		}
	}

	p, err := postgres.Connect(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := postgres.Migrate(ctx, p); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}
