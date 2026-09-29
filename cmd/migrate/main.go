package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"bmj-backend/internal/config"
	"bmj-backend/internal/migrate"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: migrate up|down|version")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch os.Args[1] {
	case "up":
		applied, err := migrate.Up(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		if len(applied) == 0 {
			fmt.Println("up to date")
			return
		}
		for _, migration := range applied {
			fmt.Printf("applied %06d_%s\n", migration.Version, migration.Name)
		}
	case "down":
		migration, err := migrate.Down(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("rolled back %06d_%s\n", migration.Version, migration.Name)
	case "version":
		version, err := migrate.Version(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(version)
	default:
		log.Fatal("usage: migrate up|down|version")
	}
}
