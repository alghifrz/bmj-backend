package response

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type successResponse struct {
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// JSONError writes the shared API error body and stops the handler chain.
func JSONError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// JSONData writes a successful JSON body.
func JSONData(c *gin.Context, data any) {
	JSONStatus(c, http.StatusOK, data)
}

// JSONStatus writes a successful JSON body with an explicit status code.
func JSONStatus(c *gin.Context, status int, data any) {
	c.JSON(status, successResponse{
		Data:    data,
		Message: "Success",
	})
}

// JSONConstraint maps unique and foreign-key failures to API errors.
// It reports whether the error was handled.
func JSONConstraint(c *gin.Context, err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case "23505":
		if strings.Contains(pgErr.ConstraintName, "slug") {
			JSONError(c, http.StatusConflict, "SLUG_ALREADY_EXISTS", "Slug already exists")
			return true
		}
		JSONError(c, http.StatusConflict, "ALREADY_EXISTS", "Already exists")
		return true
	case "23503":
		detail := strings.ToLower(pgErr.Message + " " + pgErr.Detail)
		if strings.Contains(detail, "not present") {
			JSONError(c, http.StatusBadRequest, "CATEGORY_NOT_FOUND", "Category not found")
			return true
		}
		if strings.Contains(detail, "still referenced") {
			JSONError(c, http.StatusConflict, "CATEGORY_IN_USE", "Category is used by products")
			return true
		}
		JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
		return true
	case "23514", "22P02":
		JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
		return true
	default:
		return false
	}
}

// JSONQueryError logs a database failure without returning driver details.
func JSONQueryError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr):
		log.Printf("database query failed: %s", pgErr.Code)
	case unavailable(err):
		log.Print("database query failed: unavailable")
		JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
		return
	default:
		log.Print("database query failed")
	}

	if unavailable(err) {
		JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
		return
	}

	JSONError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
}

func unavailable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection refused") ||
		strings.Contains(message, "no such host") ||
		strings.Contains(message, "failed to connect") ||
		strings.Contains(message, "conn closed") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "network is unreachable")
}
