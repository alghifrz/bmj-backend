package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type loginResponse struct {
	Token string        `json:"token"`
	Admin adminResponse `json:"admin"`
}

// LoginHandler checks email and password and returns a token.
func LoginHandler(pool *pgxpool.Pool, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body loginRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		email := strings.ToLower(strings.TrimSpace(body.Email))
		password := body.Password
		if email == "" || password == "" || len(email) > 254 || len(password) > 72 {
			response.JSONError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
			return
		}
		if secret == "" {
			response.JSONError(c, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED", "Authentication is not configured")
			return
		}
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		admin, hash, active, err := findAdmin(ctx, pool, email)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = checkPassword(dummyPasswordHash, password)
			response.JSONError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
			return
		}
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}
		if err := checkPassword(hash, password); err != nil {
			log.Print("admin login failed")
			response.JSONError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
			return
		}
		if !active {
			response.JSONError(c, http.StatusForbidden, "ACCOUNT_INACTIVE", "Account inactive")
			return
		}

		token, err := SignToken(secret, admin, time.Now())
		if err != nil {
			response.JSONError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
			return
		}

		response.JSONData(c, loginResponse{
			Token: token,
			Admin: adminResponse{ID: admin.ID, Name: admin.Name, Email: admin.Email},
		})
	}
}

// MeHandler returns the authenticated admin.
func MeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, ok := CurrentAdmin(c)
		if !ok {
			response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}
		response.JSONData(c, adminResponse{ID: admin.ID, Name: admin.Name, Email: admin.Email})
	}
}

type accountUpdate struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UpdateHandler changes the signed-in admin name, email, or password.
func UpdateHandler(pool *pgxpool.Pool, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current, ok := CurrentAdmin(c)
		if !ok {
			response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}

		var body accountUpdate
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		name := strings.TrimSpace(body.Name)
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if name == "" || len(name) > 120 || !validAccountEmail(email) {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		if body.NewPassword != "" && !validPassword(body.NewPassword) {
			response.JSONError(c, http.StatusBadRequest, "INVALID_PASSWORD", "Invalid password")
			return
		}
		if secret == "" || pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED", "Authentication is not configured")
			return
		}

		emailChanged := email != strings.ToLower(strings.TrimSpace(current.Email))
		passwordChange := body.NewPassword != ""
		if (emailChanged || passwordChange) && strings.TrimSpace(body.CurrentPassword) == "" {
			response.JSONError(c, http.StatusBadRequest, "CURRENT_PASSWORD_REQUIRED", "Current password is required")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		if emailChanged || passwordChange || strings.TrimSpace(body.CurrentPassword) != "" {
			_, hash, active, err := findAdmin(ctx, pool, current.Email)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
				response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
				return
			}
			if err != nil {
				response.JSONQueryError(c, err)
				return
			}
			if err := checkPassword(hash, body.CurrentPassword); err != nil {
				response.JSONError(c, http.StatusBadRequest, "CURRENT_PASSWORD", "Current password is incorrect")
				return
			}
		}

		var nextHash *string
		if passwordChange {
			hash, err := HashPassword(body.NewPassword)
			if err != nil {
				response.JSONError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
				return
			}
			nextHash = &hash
		}

		if err := updateAdmin(ctx, pool, current.ID, name, email, nextHash); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				response.JSONError(c, http.StatusConflict, "EMAIL_TAKEN", "Email already exists")
				return
			}
			if errors.Is(err, pgx.ErrNoRows) {
				response.JSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
				return
			}
			response.JSONQueryError(c, err)
			return
		}

		updated := Admin{ID: current.ID, Name: name, Email: email}
		token, err := SignToken(secret, updated, time.Now())
		if err != nil {
			response.JSONError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
			return
		}
		response.JSONData(c, loginResponse{
			Token: token,
			Admin: adminResponse{ID: updated.ID, Name: updated.Name, Email: updated.Email},
		})
	}
}

func validAccountEmail(email string) bool {
	return email != "" && len(email) <= 254 && strings.Count(email, "@") == 1 && !strings.Contains(email, " ")
}

func updateAdmin(ctx context.Context, pool *pgxpool.Pool, id, name, email string, passwordHash *string) error {
	tag, err := pool.Exec(ctx, `
UPDATE admins
SET name = $2, email = $3, password_hash = COALESCE($4, password_hash)
WHERE id = $1::uuid AND is_active`, id, name, email, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// LogoutHandler confirms the client should discard its token.
func LogoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.JSONData(c, gin.H{"logged_out": true})
	}
}

func findAdmin(ctx context.Context, pool *pgxpool.Pool, email string) (Admin, string, bool, error) {
	const query = `
SELECT id::text, name, email, password_hash, is_active
FROM admins
WHERE lower(email) = lower($1)`

	var (
		admin  Admin
		hash   string
		active bool
	)
	err := pool.QueryRow(ctx, query, email).Scan(&admin.ID, &admin.Name, &admin.Email, &hash, &active)
	return admin, hash, active, err
}

// CreateAdmin inserts one active administrator.
func CreateAdmin(ctx context.Context, pool *pgxpool.Pool, name, email, password string) error {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || !strings.Contains(email, "@") || strings.Contains(email, " ") {
		return ErrInvalidAdmin
	}
	if !validPassword(password) {
		return ErrInvalidPassword
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
INSERT INTO admins (name, email, password_hash)
VALUES ($1, $2, $3)`, name, email, hash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAdminExists
		}
		return err
	}
	return nil
}

var (
	// ErrInvalidAdmin means the name or email cannot be stored.
	ErrInvalidAdmin = errors.New("name and email are required")
	// ErrInvalidPassword means the password is outside the allowed length.
	ErrInvalidPassword = errors.New("password must be 8 to 72 characters")
	// ErrAdminExists means the email is already registered.
	ErrAdminExists = errors.New("admin already exists")
)
