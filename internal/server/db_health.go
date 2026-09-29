package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errDatabaseUnavailable = errors.New("database unavailable")

const databasePingTimeout = 3 * time.Second

func databaseHealth(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := pingDatabase(c.Request.Context(), pool); err != nil {
			JSONError(c, http.StatusServiceUnavailable, codeDatabaseUnavailable, "Database unavailable")
			return
		}

		c.JSON(http.StatusOK, struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}
}

func pingDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errDatabaseUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, databasePingTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		log.Print("database health check failed")
		return errDatabaseUnavailable
	}

	return nil
}
