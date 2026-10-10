package middleware

import (
	"context"
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
	exception.ErrReservationNotFound.Code: http.StatusNotFound,
	exception.ErrReservationExpired.Code:  http.StatusGone,
	exception.ErrRouteNotFound.Code:       http.StatusNotFound,
	exception.ErrMethodNotAllowed.Code:    http.StatusMethodNotAllowed,
	exception.ErrInternalServerError.Code: http.StatusInternalServerError,
	exception.ErrServiceUnavailable.Code:  http.StatusServiceUnavailable,
}

// retryAfterSeconds is what a client is told to wait after a 503.
const retryAfterSeconds = "1"

// NewErrorHandler writes the error envelope for any error a handler
// reported with c.Error. An error that is not an *exception.Error, or one
// with no status mapping, is logged and sent as ErrInternalServerError, so
// internals never leak. A request that ran out of time, or whose client
// went away, is not a server fault: it is answered with
// ErrServiceUnavailable and a Retry-After header.
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
		if !mapped && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			slog.WarnContext(c.Request.Context(), "request timed out or was cancelled",
				"request_id", requestID, "error", err)
			e, status, mapped = exception.ErrServiceUnavailable, http.StatusServiceUnavailable, true
			c.Header("Retry-After", retryAfterSeconds)
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
