package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
)

// Pinger reports whether the database answers.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves the liveness and readiness endpoints.
type HealthHandler struct {
	DB Pinger
}

// NewHealthHandler returns a handler that checks the given database.
func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{DB: db}
}

// Health reports that the process is running. It does not touch the database.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, model.StatusResponse{Status: "ok"})
}

// Ready reports whether the service can reach the database.
func (h *HealthHandler) Ready(c *gin.Context) {
	if err := h.DB.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, model.StatusResponse{Status: "unavailable"})
		return
	}
	c.JSON(http.StatusOK, model.StatusResponse{Status: "ok"})
}
