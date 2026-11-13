package main

import (
	"context"
	"log"
	"os"
	"time"

	pgstore "github.com/QuietSugar/my-known-world/internal/store/postgres"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := pgstore.Open(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	log.Print("PostgreSQL schema is up to date")
}
