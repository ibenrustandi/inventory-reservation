package middleware

import (
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// NewRecovery turns a panic into an error for the error handler, which
// answers with the standard 500 envelope. The stack trace goes to the
// structured log, not to Gin's own plain-text writer.
func NewRecovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		slog.ErrorContext(c.Request.Context(), "panic recovered",
			"request_id", c.Writer.Header().Get(RequestIDHeader),
			"panic", fmt.Sprint(recovered),
			"stack", string(debug.Stack()),
		)
		_ = c.Error(fmt.Errorf("panic: %v", recovered))
		c.Abort()
	})
}
