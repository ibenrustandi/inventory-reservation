// Package handler holds the HTTP handlers. A handler reads the request,
// calls a use case and writes the response; it contains no business logic.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
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

// Reserve handles POST /api/v1/inventory/reserve
func (h *InventoryHandler) Reserve(c *gin.Context) {
	request := new(model.ReserveRequest)
	if err := decodeJSON(c, request); err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.InventoryUsecase.Reserve(c.Request.Context(), request)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// maxBodyBytes is far above any valid request and stops a client from
// sending an unbounded body.
const maxBodyBytes = 64 << 10

// decodeJSON reads the request body into dst. A body that is not valid
// JSON, or has a field of the wrong type, becomes an invalid-request error
// naming the problem.
func decodeJSON(c *gin.Context, dst any) error {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	err := json.NewDecoder(body).Decode(dst)
	if err == nil {
		return nil
	}

	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &typeErr) && typeErr.Field != "":
		return exception.NewInvalidRequest(model.FieldError{
			Field:  typeErr.Field,
			Reason: "must be " + jsonTypeName(typeErr.Type),
		})
	case errors.As(err, &typeErr):
		return exception.NewInvalidRequest(model.FieldError{Field: "body", Reason: "must be a JSON object"})
	case errors.Is(err, io.EOF):
		return exception.NewInvalidRequest(model.FieldError{Field: "body", Reason: "is required"})
	default:
		return exception.NewInvalidRequest(model.FieldError{Field: "body", Reason: "must be valid JSON"})
	}
}

// jsonTypeName describes the JSON type a Go field type expects.
func jsonTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Int:
		return "an integer"
	case reflect.String:
		return "a string"
	default:
		return "a " + t.String()
	}
}
