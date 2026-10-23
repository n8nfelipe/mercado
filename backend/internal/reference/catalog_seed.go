package reference

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mercado/internal/auth"
	"mercado/internal/search"
)

type CatalogInput struct {
	SellerEmail    string
	SellerPassword string
	SellerName     string
	BuyerEmail     string
	BuyerPassword  string
	BuyerName      string
}

type CatalogResult struct {
	Categories int64
	Users      int64
	Sellers    int64
	Listings   int64
}

var seedCategories = []struct {
	Slug string
	Name string
}{
	{Slug: "bicicletas", Name: "Bicicletas"},
	{Slug: "acessorios", Name: "Acessórios"},
	{Slug: "pecas-e-componentes", Name: "Peças e Componentes"},
	{Slug: "vestuario-ciclismo", Name: "Vestuário de Ciclismo"},
}

var seedListings = []struct {
	Title        string
	Description  string
	CategorySlug string
	PriceCents   int64
	Condition    string
	FreeShipping bool
	Stock        int32
}{
	{
		Title:        "Bicicleta Aro 29 em Ótimo Estado",
		Description:  "Bicicleta aro 29 com quadro de alumínio, revisada e pronta para pedalar.",
		CategorySlug: "bicicletas",
		PriceCents:   185000,
		Condition:    "used",
		FreeShipping: true,
		Stock:        3,
	},
	{
		Title:        "Capacete de Ciclismo Tamanho M Novo",
		Description:  "Capacete novo, na embalagem, com sistema de ajuste e ventilação.",
		CategorySlug: "acessorios",
		PriceCents:   12900,
		Condition:    "new",
		FreeShipping: false,
		Stock:        10,
	},
	{
		Title:        "Pedal de Plataforma em Alumínio",
		Description:  "Par de pedais de plataforma em alumínio com rolamento selado.",
		CategorySlug: "pecas-e-componentes",
		PriceCents:   8990,
		Condition:    "new",
		FreeShipping: true,
		Stock:        0,
	},
	{
		Title:        "Jersey de Ciclismo Tamanho G",
		Description:  "Jersey respirável com três bolsos traseiros e zíper completo.",
		CategorySlug: "vestuario-ciclismo",
		PriceCents:   15000,
		Condition:    "used",
		FreeShipping: false,
		Stock:        5,
	},
}

func SeedCatalog(ctx context.Context, pool *pgxpool.Pool, input CatalogInput) (CatalogResult, error) {
	sellerEmail := strings.ToLower(strings.TrimSpace(input.SellerEmail))
	buyerEmail := strings.ToLower(strings.TrimSpace(input.BuyerEmail))
	if sellerEmail == "" && input.SellerPassword != "" {
		return CatalogResult{}, fmt.Errorf("SEED_SELLER_EMAIL is required when SEED_SELLER_PASSWORD is set")
	}
	if buyerEmail == "" && input.BuyerPassword != "" {
		return CatalogResult{}, fmt.Errorf("SEED_BUYER_EMAIL is required when SEED_BUYER_PASSWORD is set")
	}
	if sellerEmail != "" && input.SellerPassword == "" {
		return CatalogResult{}, fmt.Errorf("SEED_SELLER_PASSWORD is required when SEED_SELLER_EMAIL is set")
	}
	if buyerEmail != "" && input.BuyerPassword == "" {
		return CatalogResult{}, fmt.Errorf("SEED_BUYER_PASSWORD is required when SEED_BUYER_EMAIL is set")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return CatalogResult{}, fmt.Errorf("begin catalog seed: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var result CatalogResult

	categoryIDs := make(map[string]uuid.UUID, len(seedCategories))
	for _, category := range seedCategories {
		id := seedUUID("mercado:category:" + category.Slug)
		tag, err := tx.Exec(ctx, `
			INSERT INTO categories (id, name, slug)
			VALUES ($1, $2, $3)
			ON CONFLICT (slug) DO NOTHING
		`, id, category.Name, category.Slug)
		if err != nil {
			return CatalogResult{}, fmt.Errorf("insert category %q: %w", category.Slug, err)
		}
		result.Categories += tag.RowsAffected()
		var stored uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, category.Slug).Scan(&stored); err != nil {
			return CatalogResult{}, fmt.Errorf("select category %q: %w", category.Slug, err)
		}
		categoryIDs[category.Slug] = stored
	}

	sellerID, created, err := seedUser(ctx, tx, sellerEmail, input.SellerPassword, input.SellerName)
	if err != nil {
		return CatalogResult{}, err
	}
	if created {
		result.Users++
	}
	var sellerProfileID uuid.UUID
	if sellerEmail != "" {
		profileID, profileCreated, err := seedSellerProfile(ctx, tx, sellerID, input.SellerName, input.BuyerEmail)
		if err != nil {
			return CatalogResult{}, err
		}
		if profileCreated {
			result.Sellers++
		}
		sellerProfileID = profileID
	}

	buyerID, created, err := seedUser(ctx, tx, buyerEmail, input.BuyerPassword, input.BuyerName)
	if err != nil {
		return CatalogResult{}, err
	}
	if created {
		result.Users++
	}
	if buyerEmail != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role) VALUES ($1, 'buyer')
			ON CONFLICT (user_id, role) DO NOTHING
		`, buyerID); err != nil {
			return CatalogResult{}, fmt.Errorf("insert buyer role: %w", err)
		}
	}

	if sellerProfileID != uuid.Nil {
		for index, listing := range seedListings {
			id := seedUUID("mercado:listing:" + listing.Title)
			publishedAt := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC).Add(time.Duration(index) * 24 * time.Hour)
			tag, err := tx.Exec(ctx, `
				INSERT INTO listings (
					id, seller_id, category_id, title, description, price_cents, currency,
					condition, free_shipping, status, search_text, published_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, 'BRL', $7, $8, 'published', $9, $10)
				ON CONFLICT (id) DO NOTHING
			`, id, sellerProfileID, categoryIDs[listing.CategorySlug], listing.Title, listing.Description,
				listing.PriceCents, listing.Condition, listing.FreeShipping,
				search.SearchText(listing.Title, listing.Description), publishedAt)
			if err != nil {
				return CatalogResult{}, fmt.Errorf("insert listing %q: %w", listing.Title, err)
			}
			result.Listings += tag.RowsAffected()
			if tag.RowsAffected() > 0 {
				if _, err := tx.Exec(ctx, `
					INSERT INTO inventory (listing_id, quantity) VALUES ($1, $2)
				`, id, listing.Stock); err != nil {
					return CatalogResult{}, fmt.Errorf("insert inventory for %q: %w", listing.Title, err)
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CatalogResult{}, fmt.Errorf("commit catalog seed: %w", err)
	}
	return result, nil
}

func seedUser(ctx context.Context, tx pgx.Tx, email, password, name string) (uuid.UUID, bool, error) {
	if email == "" {
		return uuid.Nil, false, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Conta de Exemplo"
	}

	var existing uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email_normalized = $1`, email).Scan(&existing)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, fmt.Errorf("select user %q: %w", email, err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("hash seed password: %w", err)
	}
	id := seedUUID("mercado:user:" + email)
	tag, err := tx.Exec(ctx, `
		INSERT INTO users (id, name, email, email_normalized, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email_normalized) DO NOTHING
	`, id, name, email, email, hash)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("insert user %q: %w", email, err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email_normalized = $1`, email).Scan(&id); err != nil {
			return uuid.Nil, false, fmt.Errorf("select existing user %q: %w", email, err)
		}
		return id, false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role) VALUES ($1, 'buyer')
		ON CONFLICT (user_id, role) DO NOTHING
	`, id); err != nil {
		return uuid.Nil, false, fmt.Errorf("insert buyer role: %w", err)
	}
	return id, true, nil
}

func seedSellerProfile(ctx context.Context, tx pgx.Tx, userID uuid.UUID, displayName, fallbackDocument string) (uuid.UUID, bool, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = "Loja de Exemplo"
	}
	var existing uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM seller_profiles WHERE user_id = $1`, userID).Scan(&existing)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, fmt.Errorf("select seller profile: %w", err)
	}

	document := digitsFor(userID.String() + fallbackDocument)
	id := seedUUID("mercado:seller:" + userID.String())
	tag, err := tx.Exec(ctx, `
		INSERT INTO seller_profiles (id, user_id, display_name, document, document_normalized, contact)
		VALUES ($1, $2, $3, $4, $4, $5)
		ON CONFLICT (user_id) DO NOTHING
	`, id, userID, displayName, document, "vendedor@example.com")
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("insert seller profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.QueryRow(ctx, `SELECT id FROM seller_profiles WHERE user_id = $1`, userID).Scan(&id); err != nil {
			return uuid.Nil, false, fmt.Errorf("select existing seller profile: %w", err)
		}
		return id, false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role) VALUES ($1, 'seller')
		ON CONFLICT (user_id, role) DO NOTHING
	`, userID); err != nil {
		return uuid.Nil, false, fmt.Errorf("insert seller role: %w", err)
	}
	return id, true, nil
}

func seedUUID(name string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(name))
}

func digitsFor(value string) string {
	sum := sha256.Sum256([]byte(value))
	numeric := binary.BigEndian.Uint64(sum[:8]) % 100000000000
	return fmt.Sprintf("%011d", numeric)
}
