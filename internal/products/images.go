package products

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
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

const (
	maxImageBytes = 5 << 20
	maxImageCount = 8
	maxAltLength  = 200
	imageTimeout  = 25 * time.Second
)

type adminImage struct {
	ID           string  `json:"id"`
	ImageURL     *string `json:"image_url"`
	AltText      string  `json:"alt_text"`
	DisplayOrder int     `json:"display_order"`
	IsPrimary    bool    `json:"is_primary"`
}

type imagePatch struct {
	AltText      *string `json:"alt_text"`
	DisplayOrder *int    `json:"display_order"`
	IsPrimary    *bool   `json:"is_primary"`
}

// AdminImageListHandler returns a product's images in gallery order.
func AdminImageListHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID, ok := productIDParam(c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()

		if !productExists(ctx, c, pool, productID) {
			return
		}
		items, err := listAdminImages(ctx, pool, productID)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		response.JSONData(c, items)
	}
}

// AdminImageCreateHandler stores an uploaded image in Supabase and records it.
func AdminImageCreateHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID, ok := productIDParam(c)
		if !ok {
			return
		}
		if objects == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Storage unavailable")
			return
		}

		data, contentType, extension, ok := readImage(c)
		if !ok {
			return
		}
		alt := strings.TrimSpace(c.Request.FormValue("alt_text"))
		if len(alt) > maxAltLength {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), imageTimeout)
		defer cancel()
		if !productExists(ctx, c, pool, productID) {
			return
		}

		var count int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM product_images WHERE product_id = $1`, productID).Scan(&count); err != nil {
			writeProductDBError(c, err)
			return
		}
		if count >= maxImageCount {
			response.JSONError(c, http.StatusBadRequest, "IMAGE_LIMIT", "Too many images")
			return
		}

		imageID, err := newImageID()
		if err != nil {
			response.JSONError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
			return
		}
		objectPath := "products/" + productID + "/" + imageID + "." + extension
		publicURL, err := objects.Upload(ctx, objectPath, contentType, data)
		if err != nil {
			log.Print("product image upload failed")
			response.JSONError(c, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Storage unavailable")
			return
		}

		_, err = pool.Exec(ctx, `
INSERT INTO product_images (id, product_id, storage_path, image_url, alt_text, display_order, is_primary)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)`,
			imageID, productID, objectPath, publicURL, alt, count, count == 0)
		if err != nil {
			_ = objects.Remove(ctx, []string{objectPath})
			writeProductDBError(c, err)
			return
		}

		item, err := findAdminImage(ctx, pool, productID, imageID)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		response.JSONStatus(c, http.StatusCreated, item)
	}
}

// AdminImageUpdateHandler changes alt text, order, or the primary image.
func AdminImageUpdateHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID, imageID, ok := imageIDs(c)
		if !ok {
			return
		}
		var body imagePatch
		if err := c.ShouldBindJSON(&body); err != nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		if body.AltText == nil && body.DisplayOrder == nil && body.IsPrimary == nil {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		if body.AltText != nil && len(strings.TrimSpace(*body.AltText)) > maxAltLength {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}
		if body.DisplayOrder != nil && (*body.DisplayOrder < 0 || *body.DisplayOrder > 1000) {
			response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), queryTimeout)
		defer cancel()
		tx, err := pool.Begin(ctx)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		defer tx.Rollback(ctx)

		if body.IsPrimary != nil && *body.IsPrimary {
			if _, err := tx.Exec(ctx, `UPDATE product_images SET is_primary = FALSE WHERE product_id = $1`, productID); err != nil {
				writeProductDBError(c, err)
				return
			}
		}

		tag, err := tx.Exec(ctx, `
UPDATE product_images
SET alt_text = CASE WHEN $3 THEN $4 ELSE alt_text END,
    display_order = CASE WHEN $5 THEN $6 ELSE display_order END,
    is_primary = CASE WHEN $7 THEN $8 ELSE is_primary END
WHERE id = $1::uuid AND product_id = $2::uuid`,
			imageID,
			productID,
			body.AltText != nil,
			altValue(body.AltText),
			body.DisplayOrder != nil,
			orderValue(body.DisplayOrder),
			body.IsPrimary != nil,
			body.IsPrimary != nil && *body.IsPrimary,
		)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		if tag.RowsAffected() == 0 {
			response.JSONError(c, http.StatusNotFound, "IMAGE_NOT_FOUND", "Image not found")
			return
		}
		if err := tx.Commit(ctx); err != nil {
			writeProductDBError(c, err)
			return
		}

		item, err := findAdminImage(ctx, pool, productID, imageID)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminImageDeleteHandler removes an image from the catalog and the bucket.
func AdminImageDeleteHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID, imageID, ok := imageIDs(c)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), imageTimeout)
		defer cancel()
		tx, err := pool.Begin(ctx)
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		defer tx.Rollback(ctx)

		var objectPath string
		var wasPrimary bool
		err = tx.QueryRow(ctx, `
DELETE FROM product_images
WHERE id = $1::uuid AND product_id = $2::uuid
RETURNING storage_path, is_primary`, imageID, productID).Scan(&objectPath, &wasPrimary)
		if errors.Is(err, pgx.ErrNoRows) {
			response.JSONError(c, http.StatusNotFound, "IMAGE_NOT_FOUND", "Image not found")
			return
		}
		if err != nil {
			writeProductDBError(c, err)
			return
		}
		if wasPrimary {
			if _, err := tx.Exec(ctx, `
UPDATE product_images
SET is_primary = TRUE
WHERE id = (
    SELECT id FROM product_images
    WHERE product_id = $1
    ORDER BY display_order ASC, created_at ASC
    LIMIT 1
)`, productID); err != nil {
				writeProductDBError(c, err)
				return
			}
		}
		if err := tx.Commit(ctx); err != nil {
			writeProductDBError(c, err)
			return
		}
		removeStoredObjects(ctx, objects, []string{objectPath})
		response.JSONData(c, gin.H{"deleted": true})
	}
}

func listImagePaths(ctx context.Context, pool *pgxpool.Pool, productID string) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT storage_path FROM product_images WHERE product_id = $1`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func removeStoredObjects(ctx context.Context, objects *storage.Client, paths []string) {
	if objects == nil || len(paths) == 0 {
		return
	}
	if err := objects.Remove(ctx, paths); err != nil {
		log.Print("product image delete failed")
	}
}

func readImage(c *gin.Context) ([]byte, string, string, bool) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.JSONError(c, http.StatusBadRequest, "IMAGE_TYPE", "Unsupported image")
		return nil, "", "", false
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
		return nil, "", "", false
	}
	if len(data) == 0 || len(data) > maxImageBytes {
		response.JSONError(c, http.StatusBadRequest, "IMAGE_TOO_LARGE", "Image is too large")
		return nil, "", "", false
	}
	contentType, extension, ok := sniffImage(data)
	if !ok {
		response.JSONError(c, http.StatusBadRequest, "IMAGE_TYPE", "Unsupported image")
		return nil, "", "", false
	}
	return data, contentType, extension, true
}

func sniffImage(data []byte) (string, string, bool) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg", "jpg", true
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png", "png", true
	case len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))):
		return "image/gif", "gif", true
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", "webp", true
	default:
		return "", "", false
	}
}

func productIDParam(c *gin.Context) (string, bool) {
	id := strings.TrimSpace(c.Param("id"))
	if !input.ValidUUID(id) {
		response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		return "", false
	}
	return id, true
}

func imageIDs(c *gin.Context) (string, string, bool) {
	productID, ok := productIDParam(c)
	if !ok {
		return "", "", false
	}
	imageID := strings.TrimSpace(c.Param("imageId"))
	if !input.ValidUUID(imageID) {
		response.JSONError(c, http.StatusNotFound, "IMAGE_NOT_FOUND", "Image not found")
		return "", "", false
	}
	return productID, imageID, true
}

func productExists(ctx context.Context, c *gin.Context, pool *pgxpool.Pool, productID string) bool {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id = $1::uuid)`, productID).Scan(&exists); err != nil {
		writeProductDBError(c, err)
		return false
	}
	if !exists {
		response.JSONError(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found")
		return false
	}
	return true
}

func listAdminImages(ctx context.Context, pool *pgxpool.Pool, productID string) ([]adminImage, error) {
	rows, err := pool.Query(ctx, `
SELECT id::text, image_url, alt_text, display_order, is_primary
FROM product_images
WHERE product_id = $1
ORDER BY display_order ASC, created_at ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]adminImage, 0)
	for rows.Next() {
		var item adminImage
		if err := rows.Scan(&item.ID, &item.ImageURL, &item.AltText, &item.DisplayOrder, &item.IsPrimary); err != nil {
			return nil, err
		}
		item.ImageURL = blankToNil(item.ImageURL)
		items = append(items, item)
	}
	return items, rows.Err()
}

func findAdminImage(ctx context.Context, pool *pgxpool.Pool, productID, imageID string) (adminImage, error) {
	var item adminImage
	err := pool.QueryRow(ctx, `
SELECT id::text, image_url, alt_text, display_order, is_primary
FROM product_images
WHERE product_id = $1 AND id = $2::uuid`, productID, imageID).Scan(&item.ID, &item.ImageURL, &item.AltText, &item.DisplayOrder, &item.IsPrimary)
	if err != nil {
		return adminImage{}, err
	}
	item.ImageURL = blankToNil(item.ImageURL)
	return item, nil
}

func altValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func orderValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
