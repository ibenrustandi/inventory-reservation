package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// NewTimeout cancels the request context after d, so a slow database call
// cannot hold a connection indefinitely.
func NewTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
