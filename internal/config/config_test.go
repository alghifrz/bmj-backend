package config

import (
	"strings"
	"testing"
)

func TestLoadDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("APP_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	t.Setenv("SUPABASE_STORAGE_BUCKET", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("CORS_ORIGIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.AppPort != "8080" {
		t.Fatalf("AppPort = %q, want 8080", cfg.AppPort)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("DatabaseURL = %q, want empty", cfg.DatabaseURL)
	}
	if cfg.SupabaseURL != "" {
		t.Fatal("SupabaseURL = set, want empty")
	}
	if cfg.SupabaseServiceRoleKey != "" {
		t.Fatal("SupabaseServiceRoleKey = set, want empty")
	}
	if cfg.SupabaseStorageBucket != "product-images" {
		t.Fatalf("SupabaseStorageBucket = %q, want product-images", cfg.SupabaseStorageBucket)
	}
	if cfg.JWTSecret != "" {
		t.Fatal("JWTSecret = set, want empty")
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Fatalf("CORSOrigins = %#v, want [http://localhost:3000]", cfg.CORSOrigins)
	}
}

func TestLoadSupabaseSettings(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SUPABASE_URL", "https://example.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "test-service-role-key")
	t.Setenv("SUPABASE_STORAGE_BUCKET", "product-images")
	t.Setenv("CORS_ORIGIN", "http://localhost:3000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatal("DatabaseURL was not loaded from the environment")
	}
	if cfg.SupabaseURL != "https://example.supabase.co" {
		t.Fatalf("SupabaseURL = %q", cfg.SupabaseURL)
	}
	if cfg.SupabaseServiceRoleKey != "test-service-role-key" {
		t.Fatal("SupabaseServiceRoleKey was not loaded from the environment")
	}
	if cfg.SupabaseStorageBucket != "product-images" {
		t.Fatalf("SupabaseStorageBucket = %q", cfg.SupabaseStorageBucket)
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("JWT_SECRET", "too-short")
	t.Setenv("CORS_ORIGIN", "http://localhost:3000")

	if _, err := Load(); err == nil {
		t.Fatal("expected a short JWT_SECRET to be rejected")
	}
}

func TestLoadProductionRejectsUnrestrictedCORS(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("CORS_ORIGIN", "*")

	if _, err := Load(); err == nil {
		t.Fatal("expected production wildcard CORS to be rejected")
	}
}

func TestLoadProductionRequiresOrigin(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("CORS_ORIGIN", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing production CORS_ORIGIN to be rejected")
	}
}

func TestLoadProductionAllowsExplicitOrigins(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", strings.Repeat("k", 32))
	t.Setenv("CORS_ORIGIN", "https://www.example.com, https://admin.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := []string{"https://www.example.com", "https://admin.example.com"}
	if len(cfg.CORSOrigins) != len(want) {
		t.Fatalf("CORSOrigins = %#v, want %#v", cfg.CORSOrigins, want)
	}
	for i := range want {
		if cfg.CORSOrigins[i] != want[i] {
			t.Fatalf("CORSOrigins = %#v, want %#v", cfg.CORSOrigins, want)
		}
	}
}
