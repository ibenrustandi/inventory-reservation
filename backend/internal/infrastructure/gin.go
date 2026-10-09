package infrastructure

import "github.com/gin-gonic/gin"

// NewGin returns the Gin engine with the service's settings. Middleware
// and routes are attached by route.RegisterRoute.
func NewGin() *gin.Engine {
	app := gin.New()
	// Answer 405 for a known path with the wrong method, not 404.
	app.HandleMethodNotAllowed = true
	return app
}
