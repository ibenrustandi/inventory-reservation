package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/exception"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/model"
)

// statusByCode is the one place that decides which HTTP status each error
// code gets. Every error defined in the exception package needs an entry.
var statusByCode = map[string]int{
	exception.ErrInvalidRequest.Code:      http.StatusBadRequest,
	exception.ErrItemNotFound.Code:        http.StatusNotFound,
	exception.ErrInsufficientStock.Code:   http.StatusConflict,
	exception.ErrRouteNotFound.Code:       http.StatusNotFound,
	exception.ErrMethodNotAllowed.Code:    http.StatusMethodNotAllowed,
	exception.ErrInternalServerError.Code: http.StatusInternalServerError,
}

// NewErrorHandler writes the error envelope for any error a handler
// reported with c.Error. An error that is not an *exception.Error, or one
// with no status mapping, is logged and sent as ErrInternalServerError, so
// internals never leak.
func NewErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		requestID := c.Writer.Header().Get(RequestIDHeader)
		err := c.Errors.Last().Err

		var e *exception.Error
		status, mapped := 0, false
		if errors.As(err, &e) {
			status, mapped = statusByCode[e.Code]
		}
		if !mapped {
			slog.ErrorContext(c.Request.Context(), "request failed",
				"request_id", requestID, "error", err)
			e, status = exception.ErrInternalServerError, http.StatusInternalServerError
		}

		c.JSON(status, model.ErrorResponse{
			Status:    "error",
			Error:     model.ErrorBody{Code: e.Code, Message: e.Message, Details: e.Details},
			RequestID: requestID,
		})
	}
}
