package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxConns          = 10
	minConns          = 0
	maxConnLifetime   = time.Hour
	maxConnIdleTime   = 30 * time.Minute
	healthCheckPeriod = time.Minute
)

// NewPool creates a PostgreSQL connection pool from a database URL.
// The caller owns the pool and must close it.
// The URL supplies the host, credentials, and TLS settings. Simple protocol is
// used so the pool also works through Supabase's transaction pooler.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database url")
	}

	poolConfig.MaxConns = maxConns
	poolConfig.MinConns = minConns
	poolConfig.MaxConnLifetime = maxConnLifetime
	poolConfig.MaxConnIdleTime = maxConnIdleTime
	poolConfig.HealthCheckPeriod = healthCheckPeriod
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	return pool, nil
}
