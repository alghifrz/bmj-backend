package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"bmj-backend/internal/config"
	"bmj-backend/internal/database"
	"bmj-backend/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := openDatabase(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Print("database pool was not created")
	}
	if pool != nil {
		defer pool.Close()
	}

	srv, err := server.New(cfg, pool)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func openDatabase(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	return database.NewPool(ctx, databaseURL)
}
