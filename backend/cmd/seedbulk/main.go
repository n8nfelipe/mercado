package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mercado/internal/reference"
)

func main() {
	defaultSeller := os.Getenv("SEED_SELLER_EMAIL")
	if defaultSeller == "" {
		defaultSeller = "vendedor@example.com"
	}

	count := flag.Int("count", 100000, "number of listings to create")
	batch := flag.Int("batch", 1000, "rows per insert batch")
	seller := flag.String("seller", defaultSeller, "seller account email")
	timeout := flag.Duration("timeout", 15*time.Minute, "database timeout")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fail("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fail("open database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fail("ping database: %v", err)
	}

	result, err := reference.SeedBulkListings(ctx, pool, reference.BulkInput{
		Count:       *count,
		BatchSize:   *batch,
		SellerEmail: *seller,
	})
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("requested=%d inserted=%d seller=%s duration=%s\n",
		result.Requested, result.Inserted, result.Seller, result.Duration.Round(time.Millisecond))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seedbulk: "+format+"\n", args...)
	os.Exit(1)
}
