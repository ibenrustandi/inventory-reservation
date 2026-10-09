package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// NewLogger writes one structured log line per request.
func NewLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.InfoContext(c.Request.Context(), "request",
			"request_id", c.Writer.Header().Get(RequestIDHeader),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
