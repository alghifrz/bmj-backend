package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"bmj-backend/internal/auth"
	"bmj-backend/internal/config"
	"bmj-backend/internal/database"

	"github.com/gin-gonic/gin"
)

func TestAdminCatalogAgainstDatabase(t *testing.T) {
	if os.Getenv("BMJ_INTEGRATION") != "1" {
		t.Skip("set BMJ_INTEGRATION=1 to run admin catalog checks")
	}

	gin.SetMode(gin.TestMode)
	t.Chdir("../..")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" {
		t.Fatal("database and JWT settings are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatal("database pool was not created")
	}
	defer pool.Close()

	var admin auth.Admin
	err = pool.QueryRow(ctx, `
SELECT id::text, name, email
FROM admins
WHERE lower(email) = lower($1) AND is_active`, "you@example.com").Scan(&admin.ID, &admin.Name, &admin.Email)
	if err != nil {
		t.Fatal("active admin you@example.com was not found")
	}

	token, err := auth.SignToken(cfg.JWTSecret, admin, time.Now())
	if err != nil {
		t.Fatal("token was not created")
	}

	srv, err := New(cfg, pool)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	handler := srv.Handler()

	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000"), ".", "")
	slug := "bmj-crud-" + suffix

	var categoryID, productID, reviewID, locationID string
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		if productID != "" {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM products WHERE id = $1::uuid`, productID)
		}
		if categoryID != "" {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM categories WHERE id = $1::uuid`, categoryID)
		}
		if reviewID != "" {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM reviews WHERE id = $1::uuid`, reviewID)
		}
		if locationID != "" {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM store_locations WHERE id = $1::uuid`, locationID)
		}
	})

	status, payload := callJSON(handler, http.MethodPost, "/api/v1/admin/categories", token, map[string]any{
		"name":      "BMJ CRUD " + suffix,
		"slug":      slug,
		"is_active": true,
	})
	requireStatus(t, status, payload, http.StatusCreated)
	categoryID = dataID(t, payload)

	status, payload = callJSON(handler, http.MethodPost, "/api/v1/admin/products", token, map[string]any{
		"category_id":   categoryID,
		"name":          "BMJ CRUD Product " + suffix,
		"slug":          slug,
		"price":         150000.5,
		"price_visible": false,
		"availability":  "Available",
		"is_published":  false,
	})
	requireStatus(t, status, payload, http.StatusCreated)
	productID = dataID(t, payload)
	if dataField(t, payload, "price") == nil {
		t.Fatal("admin create omitted the stored price")
	}

	status, payload = callJSON(handler, http.MethodGet, "/api/v1/products/"+slug, "", nil)
	requireStatus(t, status, payload, http.StatusNotFound)

	status, payload = callJSON(handler, http.MethodPatch, "/api/v1/admin/products/"+productID, token, map[string]any{
		"is_published": true,
		"is_featured":  true,
	})
	requireStatus(t, status, payload, http.StatusOK)
	if dataField(t, payload, "price") == nil {
		t.Fatal("admin update omitted the stored price")
	}

	status, payload = callJSON(handler, http.MethodGet, "/api/v1/products/"+slug, "", nil)
	requireStatus(t, status, payload, http.StatusOK)
	if dataField(t, payload, "price") != nil {
		t.Fatal("public product returned a hidden price")
	}

	status, payload = callJSON(handler, http.MethodPost, "/api/v1/admin/reviews", token, map[string]any{
		"customer_name": "BMJ CRUD",
		"review_text":   "Temporary review " + suffix,
		"rating":        5,
		"is_published":  false,
	})
	requireStatus(t, status, payload, http.StatusCreated)
	reviewID = dataID(t, payload)

	status, payload = callJSON(handler, http.MethodPatch, "/api/v1/admin/reviews/"+reviewID, token, map[string]any{
		"is_published": true,
	})
	requireStatus(t, status, payload, http.StatusOK)

	status, payload = callJSON(handler, http.MethodGet, "/api/v1/admin/store", token, nil)
	requireStatus(t, status, payload, http.StatusOK)
	originalFooter := dataField(t, payload, "footer_text")
	locations, _ := dataField(t, payload, "locations").([]any)
	if len(locations) == 0 {
		t.Fatal("expected existing store locations")
	}
	first, _ := locations[0].(map[string]any)
	existingLocationID, _ := first["id"].(string)
	originalOrder := first["display_order"]

	status, payload = callJSON(handler, http.MethodPatch, "/api/v1/admin/store", token, map[string]any{
		"footer_text": "bmj-crud-temp",
	})
	requireStatus(t, status, payload, http.StatusOK)
	status, payload = callJSON(handler, http.MethodPatch, "/api/v1/admin/store", token, map[string]any{
		"footer_text": originalFooter,
	})
	requireStatus(t, status, payload, http.StatusOK)

	status, payload = callJSON(handler, http.MethodPatch, "/api/v1/admin/store/locations/"+existingLocationID, token, map[string]any{
		"display_order": 9,
	})
	requireStatus(t, status, payload, http.StatusOK)
	status, payload = callJSON(handler, http.MethodPatch, "/api/v1/admin/store/locations/"+existingLocationID, token, map[string]any{
		"display_order": originalOrder,
	})
	requireStatus(t, status, payload, http.StatusOK)

	status, payload = callJSON(handler, http.MethodPost, "/api/v1/admin/store/locations", token, map[string]any{
		"name":         "BMJ CRUD Location " + suffix,
		"address":      "Temporary address",
		"is_published": false,
	})
	requireStatus(t, status, payload, http.StatusCreated)
	locationID = dataID(t, payload)

	status, payload = callJSON(handler, http.MethodDelete, "/api/v1/admin/store/locations/"+locationID, token, nil)
	requireStatus(t, status, payload, http.StatusOK)
	locationID = ""

	status, payload = callJSON(handler, http.MethodDelete, "/api/v1/admin/products/"+productID, token, nil)
	requireStatus(t, status, payload, http.StatusOK)
	productID = ""

	status, payload = callJSON(handler, http.MethodDelete, "/api/v1/admin/categories/"+categoryID, token, nil)
	requireStatus(t, status, payload, http.StatusOK)
	categoryID = ""

	status, payload = callJSON(handler, http.MethodDelete, "/api/v1/admin/reviews/"+reviewID, token, nil)
	requireStatus(t, status, payload, http.StatusOK)
	reviewID = ""
}

func callJSON(handler http.Handler, method, path, token string, body any) (int, map[string]any) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, map[string]any{"error": map[string]any{"code": "ENCODE"}}
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	payload := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec.Code, payload
}

func requireStatus(t *testing.T, status int, payload map[string]any, want int) {
	t.Helper()
	if status == want {
		return
	}
	code := ""
	if errBody, ok := payload["error"].(map[string]any); ok {
		code, _ = errBody["code"].(string)
	}
	t.Fatalf("status = %d, want %d, error = %s", status, want, code)
}

func dataID(t *testing.T, payload map[string]any) string {
	t.Helper()
	id, _ := dataField(t, payload, "id").(string)
	if id == "" {
		t.Fatal("response did not include an id")
	}
	return id
}

func dataField(t *testing.T, payload map[string]any, key string) any {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatal("response did not include data")
	}
	return data[key]
}
