package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"bmj-backend/internal/auth"
	"bmj-backend/internal/config"
	"bmj-backend/internal/database"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "create" {
		log.Fatal("usage: admin create -name NAME -email EMAIL")
	}

	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "admin name")
	email := fs.String("email", "", "admin email")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		log.Fatal("ADMIN_PASSWORD is required")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database pool was not created")
	}
	defer pool.Close()

	if err := auth.CreateAdmin(ctx, pool, *name, *email, password); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("created admin %s\n", *email)
}
