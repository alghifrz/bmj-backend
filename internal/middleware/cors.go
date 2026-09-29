package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	allowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	allowHeaders = "Origin, Content-Type, Accept, Authorization"
	maxAge       = "600"
)

// CORS allows browser requests from the configured frontend origins.
// A "*" entry permits any origin and is rejected for production by config loading.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		switch {
		case origin != "" && allowAll:
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Vary", "Origin")
		case origin != "" && originAllowed(allowed, origin):
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}

		if origin != "" && (allowAll || originAllowed(allowed, origin)) {
			c.Header("Access-Control-Allow-Methods", allowMethods)
			c.Header("Access-Control-Allow-Headers", allowHeaders)
			c.Header("Access-Control-Max-Age", maxAge)
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func originAllowed(allowed map[string]struct{}, origin string) bool {
	_, ok := allowed[origin]
	return ok
}
