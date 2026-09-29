package server

import (
	"log"
	"net/http"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
)

const (
	codeInternalServerError = "INTERNAL_SERVER_ERROR"
	codeNotFound            = "NOT_FOUND"
	codeDatabaseUnavailable = "DATABASE_UNAVAILABLE"
)

// JSONError writes the shared API error body and stops the handler chain.
func JSONError(c *gin.Context, status int, code, message string) {
	response.JSONError(c, status, code, message)
}

func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			log.Printf("panic recovered: %v", recovered)
			if c.Writer.Written() {
				c.Abort()
				return
			}

			JSONError(c, http.StatusInternalServerError, codeInternalServerError, "Internal server error")
		}()

		c.Next()
	}
}
