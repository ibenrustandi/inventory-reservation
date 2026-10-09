// Command web runs the inventory reservation HTTP service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ibenrustandi/inventory-reservation/backend/config"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/route"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/infrastructure"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/repository"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
)

func main() {
	infrastructure.NewLogger()

	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()

	db, err := infrastructure.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()

	if err := infrastructure.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	slog.Info("database migrated")

	inventoryRepository := repository.NewInventoryRepository(db)
	inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository)
	inventoryHandler := handler.NewInventoryHandler(inventoryUsecase)
	healthHandler := handler.NewHealthHandler(db)

	app := infrastructure.NewGin()
	route.RegisterRoute(app, inventoryHandler, healthHandler)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("server listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}
