package reviews

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

type reviewResponse struct {
	ID                         string    `json:"id"`
	CustomerName               string    `json:"customer_name"`
	CustomerRoleOrOrganization *string   `json:"customer_role_or_organization"`
	ReviewText                 string    `json:"review_text"`
	Rating                     *int      `json:"rating"`
	CustomerImage              *string   `json:"customer_image"`
	IsFeatured                 bool      `json:"is_featured"`
	CreatedAt                  time.Time `json:"created_at"`
}

// ListHandler returns published reviews.
func ListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		featured, err := normalizeFeatured(c.Query("featured"))
		if err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_QUERY", "Invalid query")
			return
		}
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		items, err := listReviews(ctx, pool, featured)
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}

		response.JSONData(c, items)
	}
}

func listReviews(ctx context.Context, pool *pgxpool.Pool, featured string) ([]reviewResponse, error) {
	const listSQL = `
SELECT
    id::text,
    customer_name,
    customer_role_or_organization,
    review_text,
    rating,
    customer_image,
    is_featured,
    created_at
FROM reviews
WHERE is_published
  AND (
    $1 = ''
    OR ($1 = 'true' AND is_featured)
    OR ($1 = 'false' AND NOT is_featured)
  )
ORDER BY display_order ASC, created_at DESC`

	rows, err := pool.Query(ctx, listSQL, featured)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]reviewResponse, 0)
	for rows.Next() {
		var item reviewResponse
		if err := rows.Scan(
			&item.ID,
			&item.CustomerName,
			&item.CustomerRoleOrOrganization,
			&item.ReviewText,
			&item.Rating,
			&item.CustomerImage,
			&item.IsFeatured,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.CustomerRoleOrOrganization = blankToNil(item.CustomerRoleOrOrganization)
		item.CustomerImage = blankToNil(item.CustomerImage)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func normalizeFeatured(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case "true", "1":
		return "true", nil
	case "false", "0":
		return "false", nil
	default:
		return "", fmt.Errorf("invalid featured")
	}
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
