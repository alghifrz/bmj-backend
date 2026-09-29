package products

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListHandlerRejectsInvalidQueryWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/products", ListHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/products?sort=cheapest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "INVALID_QUERY" {
		t.Fatalf("error.code = %q", body.Error.Code)
	}
}

func TestGetHandlerUnavailableWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/products/:slug", GetHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/products/tensimeter", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
