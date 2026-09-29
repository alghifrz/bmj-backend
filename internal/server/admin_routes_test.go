package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminCatalogRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newTestServer(t)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodPatch, "/api/v1/admin/me"},
		{http.MethodGet, "/api/v1/admin/dashboard"},
		{http.MethodGet, "/api/v1/admin/products"},
		{http.MethodPost, "/api/v1/admin/products"},
		{http.MethodPatch, "/api/v1/admin/products/00000000-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/admin/products/00000000-0000-4000-8000-000000000001"},
		{http.MethodGet, "/api/v1/admin/products/00000000-0000-4000-8000-000000000001/images"},
		{http.MethodPost, "/api/v1/admin/products/00000000-0000-4000-8000-000000000001/images"},
		{http.MethodPatch, "/api/v1/admin/products/00000000-0000-4000-8000-000000000001/images/00000000-0000-4000-8000-000000000002"},
		{http.MethodDelete, "/api/v1/admin/products/00000000-0000-4000-8000-000000000001/images/00000000-0000-4000-8000-000000000002"},
		{http.MethodGet, "/api/v1/admin/categories"},
		{http.MethodPost, "/api/v1/admin/categories"},
		{http.MethodPatch, "/api/v1/admin/categories/00000000-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/admin/categories/00000000-0000-4000-8000-000000000001"},
		{http.MethodPost, "/api/v1/admin/categories/00000000-0000-4000-8000-000000000001/image"},
		{http.MethodDelete, "/api/v1/admin/categories/00000000-0000-4000-8000-000000000001/image"},
		{http.MethodGet, "/api/v1/admin/reviews"},
		{http.MethodPost, "/api/v1/admin/reviews"},
		{http.MethodPatch, "/api/v1/admin/reviews/00000000-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/admin/reviews/00000000-0000-4000-8000-000000000001"},
		{http.MethodPost, "/api/v1/admin/reviews/00000000-0000-4000-8000-000000000001/image"},
		{http.MethodDelete, "/api/v1/admin/reviews/00000000-0000-4000-8000-000000000001/image"},
		{http.MethodGet, "/api/v1/admin/store"},
		{http.MethodPatch, "/api/v1/admin/store"},
		{http.MethodPost, "/api/v1/admin/store/locations"},
		{http.MethodPatch, "/api/v1/admin/store/locations/00000000-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/admin/store/locations/00000000-0000-4000-8000-000000000001"},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			req := httptest.NewRequest(request.method, request.path, nil)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}
