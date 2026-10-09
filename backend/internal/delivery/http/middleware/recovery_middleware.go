package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// NewRecovery turns a panic into an error for the error handler, which
// answers with the standard 500 envelope.
func NewRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		_ = c.Error(fmt.Errorf("panic: %v", recovered))
		c.Abort()
	})
}
