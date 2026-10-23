package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mercado/internal/reference"
)

func main() {
	defaultSeed := int64(1)
	if raw := os.Getenv("SEED"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			fail("SEED must be an integer, got %q", raw)
		}
		defaultSeed = value
	}

	seed := flag.Int64("seed", defaultSeed, "deterministic seed")
	timeout := flag.Duration("timeout", 60*time.Second, "database timeout")
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

	result, err := reference.Seed(ctx, pool, *seed)
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("seed=%d currencies=%d countries=%d locales=%d fingerprint=%s\n",
		result.Seed, result.Currencies, result.Countries, result.Locales, result.Fingerprint)

	catalog, err := reference.SeedCatalog(ctx, pool, reference.CatalogInput{
		SellerEmail:    os.Getenv("SEED_SELLER_EMAIL"),
		SellerPassword: os.Getenv("SEED_SELLER_PASSWORD"),
		SellerName:     os.Getenv("SEED_SELLER_NAME"),
		BuyerEmail:     os.Getenv("SEED_BUYER_EMAIL"),
		BuyerPassword:  os.Getenv("SEED_BUYER_PASSWORD"),
		BuyerName:      os.Getenv("SEED_BUYER_NAME"),
	})
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("catalog categories=%d users=%d sellers=%d listings=%d\n",
		catalog.Categories, catalog.Users, catalog.Sellers, catalog.Listings)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seed: "+format+"\n", args...)
	os.Exit(1)
}
