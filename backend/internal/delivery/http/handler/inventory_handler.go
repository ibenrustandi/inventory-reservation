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

// Confirm handles POST /api/v1/inventory/confirm
func (h *InventoryHandler) Confirm(c *gin.Context) {
	request := new(model.ConfirmRequest)
	if err := decodeJSON(c, request); err != nil {
		_ = c.Error(err)
		return
	}

	response, err := h.InventoryUsecase.Confirm(c.Request.Context(), request)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// MaxBodyBytes is far above any valid request and stops a client from
// sending an unbounded body.
const MaxBodyBytes = 64 << 10

// decodeJSON reads the request body into dst. A body that is not valid
// JSON, has a field of the wrong type, or is followed by more data becomes
// an invalid-request error naming the problem. A body over MaxBodyBytes
// becomes a payload-too-large error.
func decodeJSON(c *gin.Context, dst any) error {
	tooLarge := exception.ErrPayloadTooLarge.WithDetails(map[string]any{"max_bytes": MaxBodyBytes})
	// Refuse a declared oversize before reading anything. A body of
	// undeclared or understated length is caught by the reader below.
	if c.Request.ContentLength > MaxBodyBytes {
		return tooLarge
	}

	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes))
	err := decoder.Decode(dst)
	if err == nil {
		// The body must be one JSON value and nothing else.
		_, err = decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			return tooLarge
		}
		return exception.NewInvalidRequest(exception.FieldError{Field: "body", Reason: "must be a single JSON object"})
	}

	var (
		typeErr *json.UnmarshalTypeError
		sizeErr *http.MaxBytesError
	)
	switch {
	case errors.As(err, &sizeErr):
		return tooLarge
	case errors.As(err, &typeErr) && typeErr.Field != "":
		return exception.NewInvalidRequest(exception.FieldError{
			Field:  typeErr.Field,
			Reason: "must be " + jsonTypeName(typeErr.Type),
		})
	case errors.As(err, &typeErr):
		return exception.NewInvalidRequest(exception.FieldError{Field: "body", Reason: "must be a JSON object"})
	case errors.Is(err, io.EOF):
		return exception.NewInvalidRequest(exception.FieldError{Field: "body", Reason: "is required"})
	default:
		return exception.NewInvalidRequest(exception.FieldError{Field: "body", Reason: "must be valid JSON"})
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
