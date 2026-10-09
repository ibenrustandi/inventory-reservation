// Package middleware holds the HTTP middleware.
package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/domain"
)

// RequestIDHeader is the header that carries the request ID.
const RequestIDHeader = "X-Request-ID"

// NewRequestID gives every request an ID, reusing a valid one sent by the
// client, and returns it in the response header.
func NewRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !domain.ValidID(id) {
			id = newRequestID()
		}
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	// crypto/rand.Read never returns an error.
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
