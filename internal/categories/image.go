package categories

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"bmj-backend/internal/input"
	"bmj-backend/internal/response"
	"bmj-backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxCategoryImageBytes = 5 << 20
	categoryImageTimeout  = 25 * time.Second
)

// AdminImageHandler stores one category image in Supabase and saves its public URL.
func AdminImageHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		if objects == nil {
			response.JSONError(c, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Storage unavailable")
			return
		}

		data, contentType, extension, ok := readCategoryImage(c)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), categoryImageTimeout)
		defer cancel()

		current, err := findAdminCategory(ctx, pool, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}

		imageID, err := newCategoryImageID()
		if err != nil {
			response.JSONError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
			return
		}
		objectPath := "categories/" + id + "/" + imageID + "." + extension
		publicURL, err := objects.Upload(ctx, objectPath, contentType, data)
		if err != nil {
			log.Print("category image upload failed")
			response.JSONError(c, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Storage unavailable")
			return
		}

		tag, err := pool.Exec(ctx, `UPDATE categories SET image_url = $2 WHERE id = $1::uuid`, id, publicURL)
		if err != nil || tag.RowsAffected() == 0 {
			_ = objects.Remove(ctx, []string{objectPath})
			if err != nil {
				writeCategoryDBError(c, err)
				return
			}
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		removeCategoryImage(ctx, objects, current.ImageURL)

		item, err := findAdminCategory(ctx, pool, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

// AdminImageDeleteHandler clears a category image from the catalog and the bucket.
func AdminImageDeleteHandler(pool *pgxpool.Pool, objects *storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if !input.ValidUUID(id) {
			response.JSONError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), categoryImageTimeout)
		defer cancel()

		current, err := findAdminCategory(ctx, pool, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		if _, err := pool.Exec(ctx, `UPDATE categories SET image_url = NULL WHERE id = $1::uuid`, id); err != nil {
			writeCategoryDBError(c, err)
			return
		}
		removeCategoryImage(ctx, objects, current.ImageURL)

		item, err := findAdminCategory(ctx, pool, id)
		if err != nil {
			writeCategoryDBError(c, err)
			return
		}
		response.JSONData(c, item)
	}
}

func removeCategoryImage(ctx context.Context, objects *storage.Client, imageURL *string) {
	if objects == nil || imageURL == nil {
		return
	}
	path, ok := objects.ObjectPath(*imageURL)
	if !ok {
		return
	}
	if err := objects.Remove(ctx, []string{path}); err != nil {
		log.Print("category image delete failed")
	}
}

func readCategoryImage(c *gin.Context) ([]byte, string, string, bool) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.JSONError(c, http.StatusBadRequest, "IMAGE_TYPE", "Unsupported image")
		return nil, "", "", false
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxCategoryImageBytes+1))
	if err != nil {
		response.JSONError(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
		return nil, "", "", false
	}
	if len(data) == 0 || len(data) > maxCategoryImageBytes {
		response.JSONError(c, http.StatusBadRequest, "IMAGE_TOO_LARGE", "Image is too large")
		return nil, "", "", false
	}
	contentType, extension, ok := sniffCategoryImage(data)
	if !ok {
		response.JSONError(c, http.StatusBadRequest, "IMAGE_TYPE", "Unsupported image")
		return nil, "", "", false
	}
	return data, contentType, extension, true
}

func sniffCategoryImage(data []byte) (string, string, bool) {
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

func newCategoryImageID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
