package categories

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

type adminCategory struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  *string   `json:"description"`
	ImageURL     *string   `json:"image_url"`
	IsActive     bool      `json:"is_active"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type categoryWrite struct {
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  *string `json:"description"`
	ImageURL     *string `json:"image_url"`
	IsActive     bool    `json:"is_active"`
	DisplayOrder int     `json:"display_order"`
}

type categoryPatch struct {
	Name         *string              `json:"name"`
	Slug         *string              `json:"slug"`
	Description  input.OptionalString `json:"description"`
	ImageURL     input.OptionalString `json:"image_url"`
	IsActive     *bool                `json:"is_active"`
	DisplayOrder *int                 `json:"display_order"`
}

const adminCategorySelect = `
SELECT id::text, name, slug, description, image_url, is_active, display_order, created_at, updated_at
FROM categories`

// AdminListHandler returns every category, including inactive ones.
func AdminListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		items, err := listAdminCategories(ctx, pool)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		response.JSONData(c, items)
	}
}

// AdminCreateHandler inserts a category.
func AdminCreateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body categoryWrite
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" || !input.ValidSlug(body.Slug) {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		var id string
		err := pool.QueryRow(ctx, `
INSERT INTO categories (name, slug, description, image_url, is_active, display_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id::text`,
			name,
			body.Slug,
			input.CleanText(body.Description),
			input.CleanText(body.ImageURL),
			body.IsActive,
			body.DisplayOrder,
		).Scan(&id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}

		item, err := findAdminCategory(ctx, pool, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		response.JSONStatus(c, http.StatusCreated, item)
	}
}

// AdminUpdateHandler applies a partial category update.
func AdminUpdateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}

		var body categoryPatch
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		update, err := categoryUpdate(body)
		if err != nil || update.Empty() {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		query, args := update.SQL("categories", id)
		tag, err := pool.Exec(ctx, query, args...)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}

		item, err := findAdminCategory(ctx, pool, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminDeleteHandler removes a category that is not used by products.
func AdminDeleteHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		var imageURL *string
		err := pool.QueryRow(ctx, `SELECT image_url FROM categories WHERE id = $1::uuid`, id).Scan(&imageURL)
		if errors.Is(err, pgx.ErrNoRows) {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}

		tag, err := pool.Exec(ctx, `DELETE FROM categories WHERE id = $1::uuid`, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		removeCategoryImage(ctx, objects, imageURL)
		response.JSONData(c, gin.H{"deleted": true})
	}
}

func categoryUpdate(body categoryPatch) (input.Update, error) {
	var update input.Update
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return update, errInvalidCategory
		}
		update.Set("name", name)
	}
	if body.Slug != nil {
		if !input.ValidSlug(*body.Slug) {
			return update, errInvalidCategory
		}
		update.Set("slug", *body.Slug)
	}
	if body.Description.Set {
		update.Set("description", input.CleanText(body.Description.Value))
	}
	if body.ImageURL.Set {
		update.Set("image_url", input.CleanText(body.ImageURL.Value))
	}
	if body.IsActive != nil {
		update.Set("is_active", *body.IsActive)
	}
	if body.DisplayOrder != nil {
		update.Set("display_order", *body.DisplayOrder)
	}
	return update, nil
}

func listAdminCategories(ctx context.Context, pool *pgxpool.Pool) ([]adminCategory, error) {
	rows, err := pool.Query(ctx, adminCategorySelect+` ORDER BY display_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]adminCategory, 0)
	for rows.Next() {
		item, err := scanAdminCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func findAdminCategory(ctx context.Context, pool *pgxpool.Pool, id string) (adminCategory, error) {
	return scanAdminCategory(pool.QueryRow(ctx, adminCategorySelect+` WHERE id = $1::uuid`, id))
}

func scanAdminCategory(row scanner) (adminCategory, error) {
	var item adminCategory
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Slug,
		&item.Description,
		&item.ImageURL,
		&item.IsActive,
		&item.DisplayOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return adminCategory{}, err
	}
	item.Description = blankToNil(item.Description)
	item.ImageURL = blankToNil(item.ImageURL)
	return item, nil
}

func writeCategoryDBError(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		return
	}
	if response.JSONConstraint(c, err) {
		return
	}
	response.JSONQueryError(c, err)
}

var errInvalidCategory = errors.New("invalid category")
