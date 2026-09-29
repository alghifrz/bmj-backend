package analytics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecordHandlerRejectsInvalidEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/events", RecordHandler(nil))

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"kind":"page_view"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRecordHandlerUnavailableWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/events", RecordHandler(nil))

	body := `{"kind":"whatsapp_click","source":"product_card","product_slug":"tensimeter","visitor_key":"11111111-1111-4111-8111-111111111111","path":"/produk"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
