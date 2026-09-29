package categories

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

type categoryResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  *string   `json:"description"`
	ImageURL     *string   `json:"image_url"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ListHandler returns active categories.
func ListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		items, err := listCategories(ctx, pool)
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}

		response.JSONData(c, items)
	}
}

// GetHandler returns one active category by slug.
func GetHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" || len(slug) > 200 {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		item, err := getCategory(ctx, pool, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}

		response.JSONData(c, item)
	}
}

func listCategories(ctx context.Context, pool *pgxpool.Pool) ([]categoryResponse, error) {
	const listSQL = `
SELECT id::text, name, slug, description, image_url, display_order, created_at, updated_at
FROM categories
WHERE is_active
ORDER BY display_order ASC, name ASC`

	rows, err := pool.Query(ctx, listSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]categoryResponse, 0)
	for rows.Next() {
		item, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func getCategory(ctx context.Context, pool *pgxpool.Pool, slug string) (categoryResponse, error) {
	const getSQL = `
SELECT id::text, name, slug, description, image_url, display_order, created_at, updated_at
FROM categories
WHERE is_active AND slug = $1`

	row := pool.QueryRow(ctx, getSQL, slug)
	return scanCategory(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCategory(row scanner) (categoryResponse, error) {
	var item categoryResponse
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Slug,
		&item.Description,
		&item.ImageURL,
		&item.DisplayOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return categoryResponse{}, err
	}
	item.Description = blankToNil(item.Description)
	item.ImageURL = blankToNil(item.ImageURL)
	return item, nil
}

func blankToNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
