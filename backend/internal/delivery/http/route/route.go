// Package route maps URLs to handlers.
package route

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/handler"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/delivery/http/middleware"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
)

// requestTimeout bounds how long a request may use the database.
const requestTimeout = 10 * time.Second

// RegisterRoute attaches the middleware and every route to the app.
func RegisterRoute(app *gin.Engine, inventoryHandler *handler.InventoryHandler, healthHandler *handler.HealthHandler) {
	// Order matters: the logger must see the status the error handler
	// writes, and the error handler must see what recovery reports.
	app.Use(
		middleware.NewRequestID(),
		middleware.NewLogger(),
		middleware.NewErrorHandler(),
		middleware.NewRecovery(),
		middleware.NewTimeout(requestTimeout),
	)

	app.NoRoute(func(c *gin.Context) { _ = c.Error(exception.ErrRouteNotFound) })
	app.NoMethod(func(c *gin.Context) { _ = c.Error(exception.ErrMethodNotAllowed) })

	app.GET("/healthz", healthHandler.Healthz)
	app.GET("/readyz", healthHandler.Readyz)

	inventoryRouter := app.Group("/api/v1/inventory")
	inventoryRouter.GET("/stock", inventoryHandler.Stock)
}
