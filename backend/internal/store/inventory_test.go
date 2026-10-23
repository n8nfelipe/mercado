package store_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"mercado/internal/auth"
	"mercado/internal/reference"
	"mercado/internal/store"
	"mercado/migrations"
)

func TestConcurrentInventoryAdjustments(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available")
	}

	ctx := context.Background()
	dsn, container := startPostgres(t, ctx)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := store.New(pool)

	if _, err := reference.Seed(ctx, pool, 1); err != nil {
		t.Fatalf("seed reference: %v", err)
	}
	hash, err := auth.HashPassword("senha-super-segura")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := repository.CreateUser(ctx, "Ana", "ana@example.com", "ana@example.com", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	profile, err := repository.CreateSellerProfile(ctx, user.ID, "Loja da Ana", "12345678901", "12345678901", "ana@example.com")
	if err != nil {
		t.Fatalf("create seller: %v", err)
	}
	category, err := repository.CreateCategory(ctx, store.CategoryInput{Name: "Bicicletas", Slug: "bicicletas", IsActive: true})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	listing, err := repository.CreateListing(ctx, profile.ID, user.ID, store.ListingInput{
		Title: "Bicicleta aro 29", Description: "Bicicleta revisada e pronta para uso.",
		CategoryID: category.ID, PriceCents: 100000, Currency: "BRL", Condition: "used", Stock: 5,
	})
	if err != nil {
		t.Fatalf("create listing: %v", err)
	}

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repository.AdjustInventory(ctx, listing.ID, profile.ID, user.ID, 1, "ajuste concorrente")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent adjustment: %v", err)
		}
	}

	var quantity int
	if err := pool.QueryRow(ctx, "SELECT quantity FROM inventory WHERE listing_id = $1", listing.ID).Scan(&quantity); err != nil {
		t.Fatalf("select inventory: %v", err)
	}
	if quantity != 25 {
		t.Fatalf("quantity = %d, want 25", quantity)
	}
	adjustments, err := repository.InventoryAdjustments(ctx, listing.ID, profile.ID)
	if err != nil {
		t.Fatalf("list adjustments: %v", err)
	}
	if len(adjustments) != workers {
		t.Fatalf("adjustments = %d, want %d", len(adjustments), workers)
	}
	if adjustments[0].CreatedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("unexpected adjustment timestamp")
	}

	if _, err := repository.AdjustInventory(ctx, listing.ID, profile.ID, user.ID, -100, "saida invalida"); err != store.ErrInsufficientStock {
		t.Fatalf("negative adjustment error = %v", err)
	}
}
