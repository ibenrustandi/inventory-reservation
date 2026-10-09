// Package httpapi wires the Gin router and its handlers.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter builds the HTTP router for the service.
func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", healthz)

	return r
}

// healthz reports that the process is running. It does not touch the database.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
