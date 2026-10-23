package store_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"mercado/internal/auth"
	"mercado/internal/reference"
	"mercado/internal/store"
	"mercado/migrations"
)

type catalogFixture struct {
	repository *store.Store
	pool       *pgxpool.Pool
	user       store.User
	profile    store.SellerProfile
}

func newCatalogFixture(t *testing.T) catalogFixture {
	t.Helper()
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
	return catalogFixture{repository: repository, pool: pool, user: user, profile: profile}
}

func (f catalogFixture) publish(t *testing.T, listing store.Listing) store.Listing {
	t.Helper()
	ctx := context.Background()
	if _, err := f.repository.SubmitListing(ctx, listing.ID, f.profile.ID, f.user.ID); err != nil {
		t.Fatalf("submit listing: %v", err)
	}
	published, err := f.repository.PublishListing(ctx, listing.ID, f.user.ID)
	if err != nil {
		t.Fatalf("publish listing: %v", err)
	}
	return published
}

func (f catalogFixture) create(t *testing.T, input store.ListingInput) store.Listing {
	t.Helper()
	listing, err := f.repository.CreateListing(context.Background(), f.profile.ID, f.user.ID, input)
	if err != nil {
		t.Fatalf("create listing: %v", err)
	}
	return listing
}

func TestListingSearchFiltersAndPagination(t *testing.T) {
	fixture := newCatalogFixture(t)
	ctx := context.Background()

	bikes, err := fixture.repository.CreateCategory(ctx, store.CategoryInput{Name: "Bicicletas", Slug: "bicicletas", IsActive: true})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	accessories, err := fixture.repository.CreateCategory(ctx, store.CategoryInput{Name: "Acessórios", Slug: "acessorios", IsActive: true})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	inactive, err := fixture.repository.CreateCategory(ctx, store.CategoryInput{Name: "Antigos", Slug: "antigos", IsActive: true})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	base := store.ListingInput{Currency: "BRL", Condition: "used", Description: "Descrição completa do produto para teste."}
	first := fixture.create(t, store.ListingInput{Title: "Bicicleta Aro 29 Usada", CategoryID: bikes.ID, PriceCents: 100000, FreeShipping: true, Stock: 1, Currency: base.Currency, Condition: base.Condition, Description: base.Description})
	second := fixture.create(t, store.ListingInput{Title: "Capacete Ótimo Estado", CategoryID: accessories.ID, PriceCents: 5000, Condition: "new", Stock: 3, Currency: base.Currency, Description: base.Description})
	third := fixture.create(t, store.ListingInput{Title: "Bicicleta Infantil", CategoryID: bikes.ID, PriceCents: 30000, Stock: 0, Currency: base.Currency, Condition: base.Condition, Description: base.Description})
	hidden := fixture.create(t, store.ListingInput{Title: "Peça Rara Antiga", CategoryID: inactive.ID, PriceCents: 7000, Stock: 1, Currency: base.Currency, Condition: base.Condition, Description: base.Description})

	fixture.publish(t, first)
	fixture.publish(t, second)
	fixture.publish(t, third)
	fixture.publish(t, hidden)
	if _, err := fixture.repository.UpdateCategory(ctx, inactive.ID, store.CategoryInput{Name: "Antigos", Slug: "antigos", IsActive: false}); err != nil {
		t.Fatalf("deactivate category: %v", err)
	}

	all, err := fixture.repository.SearchListings(ctx, store.ListingSearch{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("search all: %v", err)
	}
	if all.Total != 3 {
		t.Fatalf("published listings = %d, want 3", all.Total)
	}

	normalized, err := fixture.repository.SearchListings(ctx, store.ListingSearch{Query: "otimo", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("search normalized: %v", err)
	}
	if normalized.Total != 1 || normalized.Items[0].ID != second.ID {
		t.Fatalf("accent-insensitive search failed: %+v", normalized.Items)
	}

	byCategory, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{CategoryID: &accessories.ID, Page: 1, PageSize: 20})
	if byCategory.Total != 1 || byCategory.Items[0].ID != second.ID {
		t.Fatalf("category filter failed: %+v", byCategory.Items)
	}
	byCondition, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Condition: "new", Page: 1, PageSize: 20})
	if byCondition.Total != 1 {
		t.Fatalf("condition filter failed: %+v", byCondition.Items)
	}
	free := true
	byShipping, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{FreeShipping: &free, Page: 1, PageSize: 20})
	if byShipping.Total != 1 || byShipping.Items[0].ID != first.ID {
		t.Fatalf("free shipping filter failed: %+v", byShipping.Items)
	}
	minPrice := int64(20000)
	maxPrice := int64(60000)
	byPrice, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{MinPriceCents: &minPrice, MaxPriceCents: &maxPrice, Page: 1, PageSize: 20})
	if byPrice.Total != 1 || byPrice.Items[0].ID != third.ID {
		t.Fatalf("price filter failed: %+v", byPrice.Items)
	}
	combined, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Query: "bicicleta", CategoryID: &bikes.ID, Condition: "used", Page: 1, PageSize: 20})
	if combined.Total != 2 {
		t.Fatalf("combined filters failed: %+v", combined.Items)
	}

	ascending, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Sort: "price_asc", Page: 1, PageSize: 20})
	if len(ascending.Items) != 3 || ascending.Items[0].PriceCents != 5000 || ascending.Items[2].PriceCents != 100000 {
		t.Fatalf("price_asc order failed: %+v", ascending.Items)
	}
	descending, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Sort: "price_desc", Page: 1, PageSize: 20})
	if descending.Items[0].PriceCents != 100000 || descending.Items[2].PriceCents != 5000 {
		t.Fatalf("price_desc order failed: %+v", descending.Items)
	}
	newest, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Sort: "newest", Page: 1, PageSize: 20})
	if len(newest.Items) != 3 {
		t.Fatalf("newest order failed: %+v", newest.Items)
	}
	firstPage, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Sort: "price_asc", Page: 1, PageSize: 1})
	secondPage, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Sort: "price_asc", Page: 2, PageSize: 1})
	if len(firstPage.Items) != 1 || len(secondPage.Items) != 1 || firstPage.Items[0].ID == secondPage.Items[0].ID {
		t.Fatalf("pagination returned overlapping pages: %+v %+v", firstPage.Items, secondPage.Items)
	}
	if firstPage.TotalPages != 3 || secondPage.TotalPages != 3 {
		t.Fatalf("unexpected total pages: %d %d", firstPage.TotalPages, secondPage.TotalPages)
	}
	beyond, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Sort: "price_asc", Page: 9, PageSize: 1})
	if len(beyond.Items) != 0 || beyond.Total != 3 {
		t.Fatalf("page beyond range failed: %+v", beyond)
	}
	maxPageSize, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Page: 1, PageSize: 50})
	if maxPageSize.PageSize != 50 || len(maxPageSize.Items) != 3 {
		t.Fatalf("max page size failed: %+v", maxPageSize)
	}

	// search_text is kept normalized on write, not only on create.
	draft := fixture.create(t, store.ListingInput{Title: "Rascunho Oculto", CategoryID: bikes.ID, PriceCents: 1000, Stock: 1, Currency: base.Currency, Condition: base.Condition, Description: base.Description})
	updatedTitle := "Café Especial Torrado"
	listing, err := fixture.repository.UpdateListing(ctx, draft.ID, fixture.profile.ID, fixture.user.ID, store.ListingPatch{Title: &updatedTitle})
	if err != nil {
		t.Fatalf("update listing: %v", err)
	}
	if !strings.Contains(listing.SearchText, "cafe especial torrado") {
		t.Fatalf("search text was not maintained: %q", listing.SearchText)
	}
	matches, _ := fixture.repository.SearchListings(ctx, store.ListingSearch{Query: "cafe especial", Page: 1, PageSize: 20})
	if matches.Total != 0 {
		t.Fatalf("draft listing should stay hidden: %+v", matches.Items)
	}
	if _, err := fixture.repository.PublicListingByID(ctx, hidden.ID); !errors.Is(err, store.ErrListingNotFound) {
		t.Fatalf("inactive category listing is public: %v", err)
	}
	if _, err := fixture.repository.PublicListingByID(ctx, first.ID); err != nil {
		t.Fatalf("published listing is not public: %v", err)
	}
	if _, err := fixture.repository.PublicListingByID(ctx, draft.ID); !errors.Is(err, store.ErrListingNotFound) {
		t.Fatalf("draft listing is public: %v", err)
	}
}
