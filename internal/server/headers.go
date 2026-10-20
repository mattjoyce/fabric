package restapi

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware sets the response headers that each route
// shares. X-Content-Type-Options stops a browser from guessing a content
// type for a response. Cache-Control no-store keeps API responses, which can
// hold chats, sessions and configuration, out of browser and proxy caches.
// A handler that sets its own Cache-Control, as the chat stream does, keeps
// it. The Swagger UI pages hold no private data and keep the default caching.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		if !strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			h.Set("Cache-Control", "no-store")
		}
		c.Next()
	}
}
