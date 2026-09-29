package reviews

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

type adminReview struct {
	ID                         string    `json:"id"`
	CustomerName               string    `json:"customer_name"`
	CustomerRoleOrOrganization *string   `json:"customer_role_or_organization"`
	ReviewText                 string    `json:"review_text"`
	Rating                     *int      `json:"rating"`
	CustomerImage              *string   `json:"customer_image"`
	IsFeatured                 bool      `json:"is_featured"`
	IsPublished                bool      `json:"is_published"`
	DisplayOrder               int       `json:"display_order"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type reviewWrite struct {
	CustomerName               string  `json:"customer_name"`
	CustomerRoleOrOrganization *string `json:"customer_role_or_organization"`
	ReviewText                 string  `json:"review_text"`
	Rating                     *int    `json:"rating"`
	CustomerImage              *string `json:"customer_image"`
	IsFeatured                 bool    `json:"is_featured"`
	IsPublished                bool    `json:"is_published"`
	DisplayOrder               int     `json:"display_order"`
}

type reviewPatch struct {
	CustomerName               *string              `json:"customer_name"`
	CustomerRoleOrOrganization input.OptionalString `json:"customer_role_or_organization"`
	ReviewText                 *string              `json:"review_text"`
	Rating                     input.OptionalInt    `json:"rating"`
	CustomerImage              input.OptionalString `json:"customer_image"`
	IsFeatured                 *bool                `json:"is_featured"`
	IsPublished                *bool                `json:"is_published"`
	DisplayOrder               *int                 `json:"display_order"`
}

const adminReviewSelect = `
SELECT
    id::text,
    customer_name,
    customer_role_or_organization,
    review_text,
    rating,
    customer_image,
    is_featured,
    is_published,
    display_order,
    created_at,
    updated_at
FROM reviews`

// AdminListHandler returns every review, including unpublished ones.
func AdminListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		rows, err := pool.Query(ctx, adminReviewSelect+` ORDER BY display_order ASC, created_at DESC`)
		if err != nil {
			writeReviewDBError(c, err)
			return
		}
		defer rows.Close()

		items := make([]adminReview, 0)
		for rows.Next() {
			item, err := scanAdminReview(rows)
			if err != nil {
				writeReviewDBError(c, err)
				return
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			writeReviewDBError(c, err)
			return
		}
		response.JSONData(c, items)
	}
}

// AdminCreateHandler inserts a review.
func AdminCreateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body reviewWrite
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		name := strings.TrimSpace(body.CustomerName)
		text := strings.TrimSpace(body.ReviewText)
		if name == "" || text == "" || !validRating(body.Rating) {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		var id string
		err := pool.QueryRow(ctx, `
INSERT INTO reviews (
    customer_name, customer_role_or_organization, review_text, rating, customer_image,
    is_featured, is_published, display_order
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id::text`,
			name,
			input.CleanText(body.CustomerRoleOrOrganization),
			text,
			body.Rating,
			input.CleanText(body.CustomerImage),
			body.IsFeatured,
			body.IsPublished,
			body.DisplayOrder,
		).Scan(&id)
		if err != nil {
			writeReviewDBError(c, err)
			return
		}

		item, err := findAdminReview(ctx, pool, id)
		if err != nil {
			writeReviewDBError(c, err)
			return
		}
		response.JSONStatus(c, http.StatusCreated, item)
	}
}

// AdminUpdateHandler applies a partial review update.
func AdminUpdateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
			return
		}

		var body reviewPatch
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		update, err := reviewUpdate(body)
		if err != nil || update.Empty() {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		query, args := update.SQL("reviews", id)
		tag, err := pool.Exec(ctx, query, args...)
		if err != nil {
			writeReviewDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
			return
		}

		item, err := findAdminReview(ctx, pool, id)
		if err != nil {
			writeReviewDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminDeleteHandler removes a review and its stored customer photo.
func AdminDeleteHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		var image *string
		err := pool.QueryRow(ctx, `SELECT customer_image FROM reviews WHERE id = $1::uuid`, id).Scan(&image)
		if errors.Is(err, pgx.ErrNoRows) {
			response.JSONError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
			return
		}
		if err != nil {
			writeReviewDBError(c, err)
			return
		}

		tag, err := pool.Exec(ctx, `DELETE FROM reviews WHERE id = $1::uuid`, id)
		if err != nil {
			writeReviewDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
			return
		}
		removeReviewImage(ctx, objects, image)
		response.JSONData(c, gin.H{"deleted": true})
	}
}

func reviewUpdate(body reviewPatch) (input.Update, error) {
	var update input.Update
	if body.CustomerName != nil {
		name := strings.TrimSpace(*body.CustomerName)
		if name == "" {
			return update, errInvalidReview
		}
		update.Set("customer_name", name)
	}
	if body.CustomerRoleOrOrganization.Set {
		update.Set("customer_role_or_organization", input.CleanText(body.CustomerRoleOrOrganization.Value))
	}
	if body.ReviewText != nil {
		text := strings.TrimSpace(*body.ReviewText)
		if text == "" {
			return update, errInvalidReview
		}
		update.Set("review_text", text)
	}
	if body.Rating.Set {
		if !validRating(body.Rating.Value) {
			return update, errInvalidReview
		}
		update.Set("rating", body.Rating.Value)
	}
	if body.CustomerImage.Set {
		update.Set("customer_image", input.CleanText(body.CustomerImage.Value))
	}
	if body.IsFeatured != nil {
		update.Set("is_featured", *body.IsFeatured)
	}
	if body.IsPublished != nil {
		update.Set("is_published", *body.IsPublished)
	}
	if body.DisplayOrder != nil {
		update.Set("display_order", *body.DisplayOrder)
	}
	return update, nil
}

func validRating(rating *int) bool {
	if rating == nil {
		return true
	}
	return *rating >= 1 && *rating <= 5
}

func findAdminReview(ctx context.Context, pool *pgxpool.Pool, id string) (adminReview, error) {
	return scanAdminReview(pool.QueryRow(ctx, adminReviewSelect+` WHERE id = $1::uuid`, id))
}

type reviewScanner interface {
	Scan(dest ...any) error
}

func scanAdminReview(row reviewScanner) (adminReview, error) {
	var item adminReview
	if err := row.Scan(
		&item.ID,
		&item.CustomerName,
		&item.CustomerRoleOrOrganization,
		&item.ReviewText,
		&item.Rating,
		&item.CustomerImage,
		&item.IsFeatured,
		&item.IsPublished,
		&item.DisplayOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return adminReview{}, err
	}
	item.CustomerRoleOrOrganization = blankToNil(item.CustomerRoleOrOrganization)
	item.CustomerImage = blankToNil(item.CustomerImage)
	return item, nil
}

func writeReviewDBError(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		response.JSONError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review not found")
		return
	}
	if response.JSONConstraint(c, err) {
		return
	}
	response.JSONQueryError(c, err)
}

var errInvalidReview = errors.New("invalid review")
