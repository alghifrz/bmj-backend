package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAppEnv        = "development"
	defaultAppPort       = "8080"
	defaultCORSOrigin    = "http://localhost:3000"
	defaultStorageBucket = "product-images"
	envProduction        = "production"
	unrestrictedOrigin   = "*"
)

// Config holds runtime settings loaded from the environment.
type Config struct {
	AppEnv                 string
	AppPort                string
	DatabaseURL            string
	SupabaseURL            string
	SupabaseServiceRoleKey string
	SupabaseStorageBucket  string
	JWTSecret              string
	CORSOrigins            []string
}

// Load reads configuration from the environment.
// A local .env file is applied when present and does not override existing variables.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	appEnv := getenv("APP_ENV", defaultAppEnv)
	appPort := getenv("APP_PORT", defaultAppPort)

	rawOrigins := os.Getenv("CORS_ORIGIN")
	if strings.TrimSpace(rawOrigins) == "" && appEnv != envProduction {
		rawOrigins = defaultCORSOrigin
	}

	origins, err := parseCORSOrigins(rawOrigins, appEnv)
	if err != nil {
		return Config{}, err
	}

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if err := validateJWTSecret(jwtSecret, appEnv); err != nil {
		return Config{}, err
	}

	return Config{
		AppEnv:                 appEnv,
		AppPort:                appPort,
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SupabaseURL:            strings.TrimSpace(os.Getenv("SUPABASE_URL")),
		SupabaseServiceRoleKey: strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")),
		SupabaseStorageBucket:  getenv("SUPABASE_STORAGE_BUCKET", defaultStorageBucket),
		JWTSecret:              jwtSecret,
		CORSOrigins:            origins,
	}, nil
}

func validateJWTSecret(secret, appEnv string) error {
	if secret == "" {
		if appEnv == envProduction {
			return fmt.Errorf("JWT_SECRET is required when APP_ENV=%s", envProduction)
		}
		return nil
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return nil
}

func parseCORSOrigins(raw string, appEnv string) ([]string, error) {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if appEnv == envProduction && origin == unrestrictedOrigin {
			return nil, fmt.Errorf("unrestricted CORS is not allowed when APP_ENV=%s", envProduction)
		}
		origins = append(origins, origin)
	}

	if appEnv == envProduction && len(origins) == 0 {
		return nil, fmt.Errorf("CORS_ORIGIN is required when APP_ENV=%s", envProduction)
	}

	return origins, nil
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
