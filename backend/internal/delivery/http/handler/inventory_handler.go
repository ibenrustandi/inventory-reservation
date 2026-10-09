// Package handler holds the HTTP handlers. A handler reads the request,
// calls a use case and writes the response; it contains no business logic.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
)

// InventoryHandler serves the inventory endpoints.
type InventoryHandler struct {
	InventoryUsecase usecase.InventoryUsecase
}

// NewInventoryHandler returns a handler that calls the given use cases.
func NewInventoryHandler(inventoryUsecase usecase.InventoryUsecase) *InventoryHandler {
	return &InventoryHandler{InventoryUsecase: inventoryUsecase}
}

// Stock handles GET /api/v1/inventory/stock?item_id=...
func (h *InventoryHandler) Stock(c *gin.Context) {
	request := &model.GetStockRequest{ItemID: c.Query("item_id")}

	response, err := h.InventoryUsecase.Stock(c.Request.Context(), request)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}
