package products

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

type categoryRef struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type imageResponse struct {
	ImageURL     *string `json:"image_url"`
	AltText      string  `json:"alt_text"`
	DisplayOrder int     `json:"display_order"`
	IsPrimary    bool    `json:"is_primary"`
}

type listItem struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Slug             string         `json:"slug"`
	ShortDescription *string        `json:"short_description"`
	Brand            *string        `json:"brand"`
	Model            *string        `json:"model"`
	Price            *string        `json:"price"`
	PriceVisible     bool           `json:"price_visible"`
	Availability     *string        `json:"availability"`
	IsFeatured       bool           `json:"is_featured"`
	Category         categoryRef    `json:"category"`
	Image            *imageResponse `json:"image"`
}

type listResponse struct {
	Items []listItem `json:"items"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
	Total int        `json:"total"`
}

type detailResponse struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Slug                  string          `json:"slug"`
	ShortDescription      *string         `json:"short_description"`
	Description           *string         `json:"description"`
	Brand                 *string         `json:"brand"`
	Model                 *string         `json:"model"`
	Material              *string         `json:"material"`
	Size                  *string         `json:"size"`
	Function              *string         `json:"function"`
	IncludedComponents    *string         `json:"included_components"`
	Specifications        *string         `json:"specifications"`
	AdditionalInformation *string         `json:"additional_information"`
	Price                 *string         `json:"price"`
	PriceVisible          bool            `json:"price_visible"`
	Availability          *string         `json:"availability"`
	IsFeatured            bool            `json:"is_featured"`
	Category              categoryRef     `json:"category"`
	Images                []imageResponse `json:"images"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

const productFromSQL = `
FROM products p
JOIN categories c ON c.id = p.category_id AND c.is_active
LEFT JOIN LATERAL (
    SELECT image_url, alt_text, display_order, is_primary
    FROM product_images
    WHERE product_id = p.id
    ORDER BY is_primary DESC, display_order ASC, created_at ASC
    LIMIT 1
) img ON TRUE
`

const productWhereSQL = `
WHERE p.is_published
  AND ($1 = '' OR c.slug = $1)
  AND ($2 = '' OR lower(p.brand) = lower($2))
  AND ($3 = '' OR p.availability = $3)
  AND (
    $4 = ''
    OR p.name ILIKE $4 ESCAPE '\'
    OR COALESCE(p.brand, '') ILIKE $4 ESCAPE '\'
    OR COALESCE(p.model, '') ILIKE $4 ESCAPE '\'
    OR COALESCE(p.short_description, '') ILIKE $4 ESCAPE '\'
    OR COALESCE(p.description, '') ILIKE $4 ESCAPE '\'
  )
  AND (
    $5 = ''
    OR ($5 = 'true' AND p.is_featured)
    OR ($5 = 'false' AND NOT p.is_featured)
  )
`

// ListHandler returns published products.
func ListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := ParseListQuery(c.Request.URL.Query())
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

		items, total, err := listProducts(ctx, pool, query)
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}

		response.JSONData(c, listResponse{
			Items: items,
			Page:  query.Page,
			Limit: query.Limit,
			Total: total,
		})
	}
}

// GetHandler returns one published product by slug.
func GetHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := strings.TrimSpace(c.Param("slug"))
		if slug == "" || len(slug) > 200 {
			response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		if pool == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		item, err := getProduct(ctx, pool, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		if err != nil {
			response.JSONQueryError(c, err)
			return
		}

		response.JSONData(c, item)
	}
}

func listProducts(ctx context.Context, pool *pgxpool.Pool, query ListQuery) ([]listItem, int, error) {
	args := []any{query.Category, query.Brand, query.Availability, query.Search, query.Featured}

	var total int
	countSQL := "SELECT COUNT(*) " + productFromSQL + productWhereSQL
	if err := pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := `
SELECT
    p.id::text,
    p.name,
    p.slug,
    p.short_description,
    p.brand,
    p.model,
    CASE WHEN p.price_visible THEN p.price::text ELSE NULL END,
    p.price_visible,
    p.availability,
    p.is_featured,
    c.name,
    c.slug,
    img.image_url,
    img.alt_text,
    img.display_order,
    img.is_primary
` + productFromSQL + productWhereSQL + `
ORDER BY ` + orderBy(query.Sort) + `
LIMIT $6 OFFSET $7`

	rows, err := pool.Query(ctx, listSQL, append(args, query.Limit, query.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]listItem, 0)
	for rows.Next() {
		var (
			item         listItem
			shortDesc    *string
			brand        *string
			model        *string
			price        *string
			availability *string
			imageURL     *string
			altText      *string
			displayOrder *int
			isPrimary    *bool
		)
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Slug,
			&shortDesc,
			&brand,
			&model,
			&price,
			&item.PriceVisible,
			&availability,
			&item.IsFeatured,
			&item.Category.Name,
			&item.Category.Slug,
			&imageURL,
			&altText,
			&displayOrder,
			&isPrimary,
		); err != nil {
			return nil, 0, err
		}

		item.ShortDescription = blankToNil(shortDesc)
		item.Brand = blankToNil(brand)
		item.Model = blankToNil(model)
		item.Price = blankToNil(price)
		item.Availability = blankToNil(availability)
		if imageURL != nil || altText != nil || isPrimary != nil {
			image := imageResponse{IsPrimary: boolValue(isPrimary), DisplayOrder: intValue(displayOrder)}
			image.ImageURL = blankToNil(imageURL)
			if altText != nil {
				image.AltText = *altText
			}
			item.Image = &image
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func getProduct(ctx context.Context, pool *pgxpool.Pool, slug string) (detailResponse, error) {
	const detailSQL = `
SELECT
    p.id::text,
    p.name,
    p.slug,
    p.short_description,
    p.description,
    p.brand,
    p.model,
    p.material,
    p.size,
    p."function",
    p.included_components,
    p.specifications,
    p.additional_information,
    CASE WHEN p.price_visible THEN p.price::text ELSE NULL END,
    p.price_visible,
    p.availability,
    p.is_featured,
    c.name,
    c.slug,
    p.created_at,
    p.updated_at
FROM products p
JOIN categories c ON c.id = p.category_id AND c.is_active
WHERE p.is_published AND p.slug = $1`

	var item detailResponse
	err := pool.QueryRow(ctx, detailSQL, slug).Scan(
		&item.ID,
		&item.Name,
		&item.Slug,
		&item.ShortDescription,
		&item.Description,
		&item.Brand,
		&item.Model,
		&item.Material,
		&item.Size,
		&item.Function,
		&item.IncludedComponents,
		&item.Specifications,
		&item.AdditionalInformation,
		&item.Price,
		&item.PriceVisible,
		&item.Availability,
		&item.IsFeatured,
		&item.Category.Name,
		&item.Category.Slug,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return detailResponse{}, err
	}

	item.ShortDescription = blankToNil(item.ShortDescription)
	item.Description = blankToNil(item.Description)
	item.Brand = blankToNil(item.Brand)
	item.Model = blankToNil(item.Model)
	item.Material = blankToNil(item.Material)
	item.Size = blankToNil(item.Size)
	item.Function = blankToNil(item.Function)
	item.IncludedComponents = blankToNil(item.IncludedComponents)
	item.Specifications = blankToNil(item.Specifications)
	item.AdditionalInformation = blankToNil(item.AdditionalInformation)
	item.Price = blankToNil(item.Price)
	item.Availability = blankToNil(item.Availability)

	images, err := listImages(ctx, pool, item.ID)
	if err != nil {
		return detailResponse{}, err
	}
	item.Images = images
	return item, nil
}

func listImages(ctx context.Context, pool *pgxpool.Pool, productID string) ([]imageResponse, error) {
	const imageSQL = `
SELECT image_url, alt_text, display_order, is_primary
FROM product_images
WHERE product_id = $1::uuid
ORDER BY is_primary DESC, display_order ASC, created_at ASC`

	rows, err := pool.Query(ctx, imageSQL, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]imageResponse, 0)
	for rows.Next() {
		var image imageResponse
		if err := rows.Scan(&image.ImageURL, &image.AltText, &image.DisplayOrder, &image.IsPrimary); err != nil {
			return nil, err
		}
		image.ImageURL = blankToNil(image.ImageURL)
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return images, nil
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

func boolValue(value *bool) bool {
	return value != nil && *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
