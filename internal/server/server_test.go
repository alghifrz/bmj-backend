package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"bmj-backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestHealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv, err := New(config.Config{
		AppEnv:      "test",
		AppPort:     "8080",
		CORSOrigins: []string{"http://localhost:3000"},
	}, nil)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	tests := []struct {
		path string
	}{
		{path: "/health"},
		{path: "/api/v1/health"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
			}

			var body struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Status != "ok" {
				t.Fatalf("status field = %q, want ok", body.Status)
			}
		})
	}
}

func TestNotFoundUsesErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv, err := New(config.Config{
		AppEnv:      "test",
		AppPort:     "8080",
		CORSOrigins: []string{"http://localhost:3000"},
	}, nil)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Fatalf("error.code = %q, want NOT_FOUND", body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Fatal("expected error.message to be set")
	}
}

func TestCORSUsesConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want http://localhost:3000", got)
	}

	other := httptest.NewRequest(http.MethodGet, "/health", nil)
	other.Header.Set("Origin", "https://evil.example")
	otherRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(otherRec, other)

	if got := otherRec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	port := freePort(t)
	srv, err := New(config.Config{
		AppEnv:      "test",
		AppPort:     port,
		CORSOrigins: []string{"http://localhost:3000"},
	}, nil)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", dialErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

func TestDatabaseHealthWhenUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/db", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "DATABASE_UNAVAILABLE" {
		t.Fatalf("error.code = %q", body.Error.Code)
	}
	if body.Error.Message != "Database unavailable" {
		t.Fatalf("error.message = %q", body.Error.Message)
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "postgres") {
		t.Fatalf("database health response leaked connection details: %s", rec.Body.String())
	}

	process := httptest.NewRequest(http.MethodGet, "/health", nil)
	processRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(processRec, process)
	if processRec.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", processRec.Code)
	}
}

func TestPublicRoutesWhenDatabaseUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newTestServer(t)

	paths := []string{
		"/api/v1/products",
		"/api/v1/products/tensimeter",
		"/api/v1/categories",
		"/api/v1/categories/alat-diagnostik",
		"/api/v1/reviews",
		"/api/v1/store",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
		})
	}
}

func TestProductListRejectsInvalidSort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=cheapest", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	srv, err := New(config.Config{
		AppEnv:      "test",
		AppPort:     "8080",
		CORSOrigins: []string{"http://localhost:3000"},
	}, nil)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	return srv
}

func freePort(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}
