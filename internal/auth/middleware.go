package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RequireAdmin rejects requests that do not carry a valid active admin token.
func RequireAdmin(pool *pgxpool.Pool, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}
		if secret == "" {
			response.JSONError(c, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED", "Authentication is not configured")
			return
		}

		admin, err := ParseToken(secret, raw, time.Now())
		if err != nil {
			response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		active, err := adminIsActive(c, pool, admin.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
				return
			}
			response.JSONQueryError(c, err)
			return
		}
		if !active {
			response.JSONError(c, http.StatusForbidden, "ACCOUNT_INACTIVE", "Account inactive")
			return
		}

		c.Set(adminContextKey, admin)
		c.Next()
	}
}

// CurrentAdmin returns the admin stored by RequireAdmin.
func CurrentAdmin(c *gin.Context) (Admin, bool) {
	value, ok := c.Get(adminContextKey)
	if !ok {
		return Admin{}, false
	}
	admin, ok := value.(Admin)
	return admin, ok
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

func adminIsActive(c *gin.Context, pool *pgxpool.Pool, id string) (bool, error) {
	var active bool
	err := pool.QueryRow(c.Request.Context(), `SELECT is_active FROM admins WHERE id = $1::uuid`, id).Scan(&active)
	return active, err
}

const adminContextKey = "auth_admin"
