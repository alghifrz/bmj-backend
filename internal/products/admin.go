package products

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/input"
	"bmj-backend/internal/response"
	"bmj-backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type adminProduct struct {
	ID                    string      `json:"id"`
	CategoryID            string      `json:"category_id"`
	Name                  string      `json:"name"`
	Slug                  string      `json:"slug"`
	ShortDescription      *string     `json:"short_description"`
	Description           *string     `json:"description"`
	Brand                 *string     `json:"brand"`
	Model                 *string     `json:"model"`
	Material              *string     `json:"material"`
	Size                  *string     `json:"size"`
	Function              *string     `json:"function"`
	IncludedComponents    *string     `json:"included_components"`
	Specifications        *string     `json:"specifications"`
	AdditionalInformation *string     `json:"additional_information"`
	Price                 *string     `json:"price"`
	PriceVisible          bool        `json:"price_visible"`
	Availability          *string     `json:"availability"`
	IsPublished           bool        `json:"is_published"`
	IsFeatured            bool        `json:"is_featured"`
	DisplayOrder          int         `json:"display_order"`
	ImageURL              *string     `json:"image_url"`
	Category              categoryRef `json:"category"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

type adminListResponse struct {
	Items []adminProduct `json:"items"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
	Total int            `json:"total"`
}

type productWrite struct {
	CategoryID            string        `json:"category_id"`
	Name                  string        `json:"name"`
	Slug                  string        `json:"slug"`
	ShortDescription      *string       `json:"short_description"`
	Description           *string       `json:"description"`
	Brand                 *string       `json:"brand"`
	Model                 *string       `json:"model"`
	Material              *string       `json:"material"`
	Size                  *string       `json:"size"`
	Function              *string       `json:"function"`
	IncludedComponents    *string       `json:"included_components"`
	Specifications        *string       `json:"specifications"`
	AdditionalInformation *string       `json:"additional_information"`
	Price                 input.Decimal `json:"price"`
	PriceVisible          bool          `json:"price_visible"`
	Availability          *string       `json:"availability"`
	IsPublished           bool          `json:"is_published"`
	IsFeatured            bool          `json:"is_featured"`
	DisplayOrder          int           `json:"display_order"`
}

type productPatch struct {
	CategoryID            *string              `json:"category_id"`
	Name                  *string              `json:"name"`
	Slug                  *string              `json:"slug"`
	ShortDescription      input.OptionalString `json:"short_description"`
	Description           input.OptionalString `json:"description"`
	Brand                 input.OptionalString `json:"brand"`
	Model                 input.OptionalString `json:"model"`
	Material              input.OptionalString `json:"material"`
	Size                  input.OptionalString `json:"size"`
	Function              input.OptionalString `json:"function"`
	IncludedComponents    input.OptionalString `json:"included_components"`
	Specifications        input.OptionalString `json:"specifications"`
	AdditionalInformation input.OptionalString `json:"additional_information"`
	Price                 input.Decimal        `json:"price"`
	PriceVisible          *bool                `json:"price_visible"`
	Availability          input.OptionalString `json:"availability"`
	IsPublished           *bool                `json:"is_published"`
	IsFeatured            *bool                `json:"is_featured"`
	DisplayOrder          *int                 `json:"display_order"`
}

const adminProductSelect = `
SELECT
    p.id::text,
    p.category_id::text,
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
    p.price::text,
    p.price_visible,
    p.availability,
    p.is_published,
    p.is_featured,
    p.display_order,
    c.name,
    c.slug,
    p.created_at,
    p.updated_at,
    (
        SELECT pi.image_url
        FROM product_images pi
        WHERE pi.product_id = p.id
        ORDER BY pi.is_primary DESC, pi.display_order ASC, pi.created_at ASC
        LIMIT 1
    )
FROM products p
JOIN categories c ON c.id = p.category_id`

// AdminListHandler returns every product, including unpublished ones.
func AdminListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := ParseListQuery(c.Request.URL.Query())
		if err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_QUERY", "Invalid query")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		total, items, err := listAdminProducts(ctx, pool, query)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		response.JSONData(c, adminListResponse{
			Items: items,
			Page:  query.Page,
			Limit: query.Limit,
			Total: total,
		})
	}
}

// AdminCreateHandler inserts a product.
func AdminCreateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body productWrite
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		availability, err := normalizeWrite(body.CategoryID, body.Name, body.Slug, body.Price, body.Availability)
		if err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		item, err := insertProduct(ctx, pool, body, availability)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		response.JSONStatus(c, http.StatusCreated, item)
	}
}

// AdminUpdateHandler applies a partial product update.
func AdminUpdateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}

		var body productPatch
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		update, err := productUpdate(body)
		if err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		if update.Empty() {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		query, args := update.SQL("products", id)
		tag, err := pool.Exec(ctx, query, args...)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}

		item, err := findAdminProduct(ctx, pool, id)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminDeleteHandler removes a product, its image rows, and the stored files.
func AdminDeleteHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), imageTimeout)
		defer cancel()

		paths, err := listImagePaths(ctx, pool, id)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		tag, err := pool.Exec(ctx, `DELETE FROM products WHERE id = $1::uuid`, id)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
			return
		}
		removeStoredObjects(ctx, objects, paths)
		response.JSONData(c, gin.H{"deleted": true})
	}
}

func normalizeWrite(categoryID, name, slug string, price input.Decimal, availability *string) (string, error) {
	if !input.ValidUUID(categoryID) || strings.TrimSpace(name) == "" || !input.ValidSlug(slug) || !price.Valid() {
		return "", errInvalidProduct
	}
	return normalizeOptionalAvailability(availability)
}

func normalizeOptionalAvailability(availability *string) (string, error) {
	if availability == nil {
		return "", nil
	}
	return input.NormalizeAvailability(*availability)
}

func productUpdate(body productPatch) (input.Update, error) {
	var update input.Update
	if body.CategoryID != nil {
		if !input.ValidUUID(*body.CategoryID) {
			return update, errInvalidProduct
		}
		update.SetCast("category_id", "uuid", *body.CategoryID)
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return update, errInvalidProduct
		}
		update.Set("name", name)
	}
	if body.Slug != nil {
		if !input.ValidSlug(*body.Slug) {
			return update, errInvalidProduct
		}
		update.Set("slug", *body.Slug)
	}
	setOptionalText(&update, "short_description", body.ShortDescription)
	setOptionalText(&update, "description", body.Description)
	setOptionalText(&update, "brand", body.Brand)
	setOptionalText(&update, "model", body.Model)
	setOptionalText(&update, "material", body.Material)
	setOptionalText(&update, "size", body.Size)
	setOptionalText(&update, `"function"`, body.Function)
	setOptionalText(&update, "included_components", body.IncludedComponents)
	setOptionalText(&update, "specifications", body.Specifications)
	setOptionalText(&update, "additional_information", body.AdditionalInformation)
	if body.Price.Set {
		if !body.Price.Valid() {
			return update, errInvalidProduct
		}
		update.SetCast("price", "numeric", body.Price.Value())
	}
	if body.PriceVisible != nil {
		update.Set("price_visible", *body.PriceVisible)
	}
	if body.Availability.Set {
		raw := ""
		if body.Availability.Value != nil {
			raw = *body.Availability.Value
		}
		availability, err := input.NormalizeAvailability(raw)
		if err != nil {
			return update, err
		}
		update.Set("availability", nullableString(availability))
	}
	if body.IsPublished != nil {
		update.Set("is_published", *body.IsPublished)
	}
	if body.IsFeatured != nil {
		update.Set("is_featured", *body.IsFeatured)
	}
	if body.DisplayOrder != nil {
		update.Set("display_order", *body.DisplayOrder)
	}
	return update, nil
}

func setOptionalText(update *input.Update, column string, value input.OptionalString) {
	if !value.Set {
		return
	}
	update.Set(column, input.CleanText(value.Value))
}

func adminOrderBy(sort string) string {
	switch sort {
	case "price_asc":
		return "p.price ASC NULLS LAST, p.name ASC, p.id ASC"
	case "price_desc":
		return "p.price DESC NULLS LAST, p.name ASC, p.id ASC"
	default:
		return orderBy(sort)
	}
}

func insertProduct(ctx context.Context, pool *pgxpool.Pool, body productWrite, availability string) (adminProduct, error) {
	const insertSQL = `
INSERT INTO products (
    category_id, name, slug, short_description, description, brand, model, material, size,
    "function", included_components, specifications, additional_information, price, price_visible,
    availability, is_published, is_featured, display_order
) VALUES (
    $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13, $14::numeric, $15,
    $16, $17, $18, $19
)
RETURNING id::text`

	var id string
	err := pool.QueryRow(ctx, insertSQL,
		body.CategoryID,
		strings.TrimSpace(body.Name),
		body.Slug,
		input.CleanText(body.ShortDescription),
		input.CleanText(body.Description),
		input.CleanText(body.Brand),
		input.CleanText(body.Model),
		input.CleanText(body.Material),
		input.CleanText(body.Size),
		input.CleanText(body.Function),
		input.CleanText(body.IncludedComponents),
		input.CleanText(body.Specifications),
		input.CleanText(body.AdditionalInformation),
		body.Price.Value(),
		body.PriceVisible,
		nullableString(availability),
		body.IsPublished,
		body.IsFeatured,
		body.DisplayOrder,
	).Scan(&id)
	if err != nil {
		return adminProduct{}, err
	}
	return findAdminProduct(ctx, pool, id)
}

func listAdminProducts(ctx context.Context, pool *pgxpool.Pool, query ListQuery) (int, []adminProduct, error) {
	const filter = `
WHERE ($1 = '' OR c.slug = $1)
  AND ($2 = '' OR lower(p.brand) = lower($2))
  AND ($3 = '' OR p.availability = $3)
  AND (
    $4 = ''
    OR p.name ILIKE $4 ESCAPE '\'
    OR COALESCE(p.brand, '') ILIKE $4 ESCAPE '\'
    OR COALESCE(p.model, '') ILIKE $4 ESCAPE '\'
  )
  AND (
    $5 = ''
    OR ($5 = 'true' AND p.is_featured)
    OR ($5 = 'false' AND NOT p.is_featured)
  )`

	args := []any{query.Category, query.Brand, query.Availability, query.Search, query.Featured}
	var total int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM products p JOIN categories c ON c.id = p.category_id `+filter, args...).Scan(&total); err != nil {
		return 0, nil, err
	}

	rows, err := pool.Query(ctx, adminProductSelect+" "+filter+` ORDER BY `+adminOrderBy(query.Sort)+` LIMIT $6 OFFSET $7`, append(args, query.Limit, query.Offset)...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	items := make([]adminProduct, 0)
	for rows.Next() {
		item, err := scanAdminProduct(rows)
		if err != nil {
			return 0, nil, err
		}
		items = append(items, item)
	}
	return total, items, rows.Err()
}

func findAdminProduct(ctx context.Context, pool *pgxpool.Pool, id string) (adminProduct, error) {
	row := pool.QueryRow(ctx, adminProductSelect+` WHERE p.id = $1::uuid`, id)
	item, err := scanAdminProduct(row)
	return item, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAdminProduct(row rowScanner) (adminProduct, error) {
	var item adminProduct
	err := row.Scan(
		&item.ID,
		&item.CategoryID,
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
		&item.IsPublished,
		&item.IsFeatured,
		&item.DisplayOrder,
		&item.Category.Name,
		&item.Category.Slug,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.ImageURL,
	)
	if err != nil {
		return adminProduct{}, err
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
	item.ImageURL = blankToNil(item.ImageURL)
	return item, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func writeProductDBError(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		return
	}
	if response.JSONConstraint(c, err) {
		return
	}
	response.JSONQueryError(c, err)
}

var errInvalidProduct = errString("invalid product")

type errString string

func (e errString) Error() string { return string(e) }
