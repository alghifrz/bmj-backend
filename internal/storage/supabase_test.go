package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bmj-backend/internal/config"
)

func TestNewDisabledWithoutSettings(t *testing.T) {
	if New(config.Config{}) != nil {
		t.Fatal("expected storage to stay disabled")
	}
}

func TestUploadAndRemove(t *testing.T) {
	var uploadedPath, deletedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("apikey") != "test-key" {
			t.Errorf("missing storage credentials")
		}
		if r.Method == http.MethodPost {
			uploadedPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			if string(body) != "image" || r.Header.Get("Content-Type") != "image/png" {
				t.Errorf("upload body was not forwarded")
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodDelete {
			payload, _ := io.ReadAll(r.Body)
			deletedBody = string(payload)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := New(config.Config{
		SupabaseURL:            server.URL,
		SupabaseServiceRoleKey: "test-key",
		SupabaseStorageBucket:  "product-images",
	})
	if client == nil {
		t.Fatal("storage client was not created")
	}

	publicURL, err := client.Upload(context.Background(), "products/11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222.png", "image/png", []byte("image"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(uploadedPath, "/storage/v1/object/product-images/products/") {
		t.Fatalf("upload path = %s", uploadedPath)
	}
	if !strings.Contains(publicURL, "/storage/v1/object/public/product-images/products/") {
		t.Fatalf("public url = %s", publicURL)
	}

	if err := client.Remove(context.Background(), []string{"products/11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222.png"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(deletedBody, "products/11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222.png") {
		t.Fatalf("delete body = %s", deletedBody)
	}
}

func TestRemoveRejectsTraversal(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(config.Config{
		SupabaseURL:            server.URL,
		SupabaseServiceRoleKey: "test-key",
		SupabaseStorageBucket:  "product-images",
	})
	if err := client.Remove(context.Background(), []string{"../secret"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("traversal path was sent to storage")
	}
}
