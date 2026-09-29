package analytics

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"bmj-backend/internal/input"
	"bmj-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 5 * time.Second

const (
	kindPageView      = "page_view"
	kindWhatsAppClick = "whatsapp_click"
	sourcePage        = "page"
	sourceProductCard = "product_card"
	sourceProductPage = "product_page"
	sourceFeatured    = "featured"
)

var (
	visitorKeyPattern = regexp.MustCompile(`^[A-Za-z0-9-]{16,64}$`)
	pathPattern       = regexp.MustCompile(`^/[A-Za-z0-9/_-]*$`)
)

type eventRequest struct {
	Kind        string `json:"kind"`
	Source      string `json:"source"`
	ProductSlug string `json:"product_slug"`
	VisitorKey  string `json:"visitor_key"`
	Path        string `json:"path"`
}

type event struct {
	Kind        string
	Source      string
	ProductSlug string
	VisitorKey  string
	Path        string
}

// RecordHandler stores a public page view or WhatsApp click.
func RecordHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body eventRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		item, ok := normalizeEvent(body)
		if !ok {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		if err := insertEvent(ctx, pool, item); err != nil {
			response.JSONQueryError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func normalizeEvent(body eventRequest) (event, bool) {
	item := event{
		Kind:        strings.TrimSpace(body.Kind),
		Source:      strings.TrimSpace(body.Source),
		ProductSlug: strings.TrimSpace(body.ProductSlug),
		VisitorKey:  strings.TrimSpace(body.VisitorKey),
		Path:        strings.TrimSpace(body.Path),
	}
	if !visitorKeyPattern.MatchString(item.VisitorKey) || !pathPattern.MatchString(item.Path) || len(item.Path) > 200 {
		return event{}, false
	}
	if item.ProductSlug != "" && !input.ValidSlug(item.ProductSlug) {
		return event{}, false
	}

	switch item.Kind {
	case kindPageView:
		if item.Source != sourcePage || item.ProductSlug != "" {
			return event{}, false
		}
	case kindWhatsAppClick:
		if item.Source != sourceProductCard && item.Source != sourceProductPage && item.Source != sourceFeatured {
			return event{}, false
		}
		if item.ProductSlug == "" {
			return event{}, false
		}
	default:
		return event{}, false
	}

	return item, true
}

func insertEvent(ctx context.Context, pool *pgxpool.Pool, item event) error {
	var productID *string
	if item.ProductSlug != "" {
		var id string
		err := pool.QueryRow(ctx, `SELECT id::text FROM products WHERE slug = $1`, item.ProductSlug).Scan(&id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			productID = &id
		}
	}

	recent, err := recentDuplicate(ctx, pool, item, productID)
	if err != nil || recent {
		return err
	}

	_, err = pool.Exec(ctx, `
INSERT INTO analytics_events (kind, source, product_id, visitor_key, path)
VALUES ($1, $2, $3::uuid, $4, $5)`,
		item.Kind,
		item.Source,
		productID,
		item.VisitorKey,
		item.Path,
	)
	return err
}

func recentDuplicate(ctx context.Context, pool *pgxpool.Pool, item event, productID *string) (bool, error) {
	window := "30 minutes"
	if item.Kind == kindWhatsAppClick {
		window = "2 seconds"
	}

	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM analytics_events
    WHERE kind = $1
      AND source = $2
      AND visitor_key = $3
      AND path = $4
      AND product_id IS NOT DISTINCT FROM $5::uuid
      AND created_at > NOW() - ($6::interval)
)`, item.Kind, item.Source, item.VisitorKey, item.Path, productID, window).Scan(&exists)
	return exists, err
}
