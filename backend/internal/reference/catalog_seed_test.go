package reference

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"mercado/migrations"
)

func TestCatalogSeedIsIdempotent(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available")
	}

	ctx := context.Background()
	dsn, container := startPostgres(t, ctx)
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

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

	if _, err := Seed(ctx, pool, 1); err != nil {
		t.Fatalf("seed reference: %v", err)
	}

	first, err := SeedCatalog(ctx, pool, CatalogInput{})
	if err != nil {
		t.Fatalf("seed catalog without accounts: %v", err)
	}
	if first.Categories != int64(len(seedCategories)) || first.Users != 0 || first.Listings != 0 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, err := SeedCatalog(ctx, pool, CatalogInput{})
	if err != nil {
		t.Fatalf("re-seed catalog without accounts: %v", err)
	}
	if second.Categories != 0 || second.Users != 0 || second.Listings != 0 {
		t.Fatalf("expected no new rows on the second run, got %+v", second)
	}

	input := CatalogInput{
		SellerEmail:    "vendedor@example.com",
		SellerPassword: "senha-super-segura",
		SellerName:     "Loja do Bairro",
		BuyerEmail:     "comprador@example.com",
		BuyerPassword:  "outra-senha-segura",
		BuyerName:      "Comprador de Exemplo",
	}
	withAccounts, err := SeedCatalog(ctx, pool, input)
	if err != nil {
		t.Fatalf("seed catalog with accounts: %v", err)
	}
	if withAccounts.Users != 2 || withAccounts.Sellers != 1 || withAccounts.Listings != int64(len(seedListings)) {
		t.Fatalf("unexpected result with accounts: %+v", withAccounts)
	}
	again, err := SeedCatalog(ctx, pool, input)
	if err != nil {
		t.Fatalf("re-seed catalog with accounts: %v", err)
	}
	if again.Users != 0 || again.Sellers != 0 || again.Listings != 0 {
		t.Fatalf("expected no new rows on the second run, got %+v", again)
	}

	assertCount(t, ctx, pool, "categories", int64(len(seedCategories)))
	assertCount(t, ctx, pool, "users", 2)
	assertCount(t, ctx, pool, "seller_profiles", 1)
	assertCount(t, ctx, pool, "listings", int64(len(seedListings)))

	var plaintext int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM users WHERE password_hash NOT LIKE '$argon2id$%'
	`).Scan(&plaintext); err != nil {
		t.Fatalf("check password hashes: %v", err)
	}
	if plaintext != 0 {
		t.Fatalf("expected all passwords to be hashed, found %d plaintext rows", plaintext)
	}

	if _, err := SeedCatalog(ctx, pool, CatalogInput{SellerEmail: "vendedor@example.com"}); err == nil {
		t.Fatal("expected an error when the seller password is missing")
	}
}

func assertCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, want int64) {
	t.Helper()
	var got int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("expected %d rows in %s, got %d", want, table, got)
	}
}
