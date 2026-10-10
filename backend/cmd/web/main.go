// Command web runs the inventory reservation HTTP service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ibenrustandi/inventory-reservation/backend/config"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/route"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/repository"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/sweeper"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
)

func main() {
	infrastructure.NewLogger()

	// The context is cancelled on Ctrl-C or when the container is stopped.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// run starts the service and blocks until ctx is cancelled and everything
// has shut down, in this order: the HTTP server (requests in flight
// finish), the sweeper (its current sweep finishes), the database pool.
// The pool closes last because the other two use it until they stop.
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := infrastructure.NewPostgres(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		db.Close()
		slog.Info("database pool closed")
	}()

	if err := infrastructure.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	slog.Info("database migrated")

	inventoryRepository := repository.NewInventoryRepository(db)
	inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, cfg.ReservationTTL)
	inventoryHandler := handler.NewInventoryHandler(inventoryUsecase)
	healthHandler := handler.NewHealthHandler(db)

	app := infrastructure.NewGin()
	route.RegisterRoute(app, inventoryHandler, healthHandler)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	// The sweeper has its own context so it keeps running while the HTTP
	// server drains, and is stopped only afterwards.
	sweeperCtx, stopSweeper := context.WithCancel(context.Background())
	sweeperStopped := make(chan struct{})
	go func() {
		defer close(sweeperStopped)
		sweeper.New(inventoryUsecase, cfg.SweepInterval).Run(sweeperCtx)
	}()

	slog.Info("server listening", "addr", listener.Addr().String())
	serveErr := infrastructure.Serve(ctx, listener, app, cfg.ShutdownTimeout)
	slog.Info("http server stopped")

	stopSweeper()
	<-sweeperStopped
	slog.Info("sweeper stopped")

	return serveErr
}
