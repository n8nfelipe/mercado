package reference

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mercado/internal/search"
)

type BulkInput struct {
	Count       int
	BatchSize   int
	SellerEmail string
}

type BulkResult struct {
	Requested int
	Inserted  int64
	Listings  int64
	Seller    string
	Duration  time.Duration
}

var bulkAdjectives = []string{
	"em Ótimo Estado", "Seminovo", "Novo na Caixa", "Revisado", "Original", "Premium",
	"Profissional", "Compacto", "Leve", "Reforçado", "Importado", "Customizado",
}

var bulkNouns = []string{
	"Bicicleta Aro 29", "Capacete", "Pedal", "Selim", "Guidão", "Roda Aro 27", "Freio a Disco",
	"Câmara de Ar", "Pneu", "Corrente", "Jersey", "Bermuda", "Luva", "Óculos", "Garrafa",
	"Bomba de Ar", "Suporte de Bicicleta", "Cadeado", "Farol", "Lanterna", "Bagageiro",
}

func SeedBulkListings(ctx context.Context, pool *pgxpool.Pool, input BulkInput) (BulkResult, error) {
	started := time.Now()
	start := BulkResult{Requested: input.Count, Seller: input.SellerEmail}

	if input.Count <= 0 {
		return start, fmt.Errorf("count must be greater than zero")
	}
	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	sellerEmail := strings.ToLower(strings.TrimSpace(input.SellerEmail))
	if sellerEmail == "" {
		return start, fmt.Errorf("seller email is required")
	}

	var sellerID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT sp.id
		FROM seller_profiles sp
		JOIN users u ON u.id = sp.user_id
		WHERE u.email_normalized = $1
	`, sellerEmail).Scan(&sellerID)
	if err != nil {
		return start, fmt.Errorf("seller %q not found (run the catalog seed first): %w", sellerEmail, err)
	}

	rows, err := pool.Query(ctx, `SELECT id FROM categories WHERE is_active ORDER BY slug`)
	if err != nil {
		return start, fmt.Errorf("select categories: %w", err)
	}
	var categoryIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return start, fmt.Errorf("scan category: %w", err)
		}
		categoryIDs = append(categoryIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return start, fmt.Errorf("iterate categories: %w", err)
	}
	if len(categoryIDs) == 0 {
		return start, fmt.Errorf("no active categories found")
	}

	base := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	random := rand.New(rand.NewSource(42))

	result := start
	for offset := 0; offset < input.Count; offset += batchSize {
		end := offset + batchSize
		if end > input.Count {
			end = input.Count
		}
		size := end - offset

		ids := make([]uuid.UUID, size)
		sellers := make([]uuid.UUID, size)
		categories := make([]uuid.UUID, size)
		titles := make([]string, size)
		descriptions := make([]string, size)
		prices := make([]int64, size)
		currencies := make([]string, size)
		conditions := make([]string, size)
		freeShipping := make([]bool, size)
		statuses := make([]string, size)
		searchTexts := make([]string, size)
		publishedAt := make([]time.Time, size)
		quantities := make([]int32, size)

		for i := 0; i < size; i++ {
			index := offset + i
			title := fmt.Sprintf("Produto %06d %s %s", index, bulkNouns[index%len(bulkNouns)], bulkAdjectives[random.Intn(len(bulkAdjectives))])
			description := fmt.Sprintf("Descrição detalhada do produto de exemplo número %d, listado para testes de catálogo e carga.", index)

			ids[i] = uuid.NewSHA1(uuid.NameSpaceURL, []byte("mercado:product:"+strconv.Itoa(index)))
			sellers[i] = sellerID
			categories[i] = categoryIDs[index%len(categoryIDs)]
			titles[i] = title
			descriptions[i] = description
			prices[i] = int64(1000 + random.Intn(500000))
			currencies[i] = "BRL"
			if index%3 == 0 {
				conditions[i] = "new"
			} else {
				conditions[i] = "used"
			}
			freeShipping[i] = random.Intn(2) == 0
			statuses[i] = "published"
			searchTexts[i] = search.SearchText(title, description)
			publishedAt[i] = base.Add(time.Duration(random.Intn(365*24)) * time.Hour)
			quantities[i] = int32(random.Intn(51))
		}

		tag, err := pool.Exec(ctx, `
			INSERT INTO listings (
				id, seller_id, category_id, title, description, price_cents, currency,
				condition, free_shipping, status, search_text, published_at
			)
			SELECT * FROM unnest(
				$1::uuid[], $2::uuid[], $3::uuid[], $4::text[], $5::text[], $6::bigint[], $7::text[],
				$8::text[], $9::bool[], $10::text[], $11::text[], $12::timestamptz[]
			)
			ON CONFLICT (id) DO NOTHING
		`, ids, sellers, categories, titles, descriptions, prices, currencies,
			conditions, freeShipping, statuses, searchTexts, publishedAt)
		if err != nil {
			return result, fmt.Errorf("insert listings batch at %d: %w", offset, err)
		}
		result.Listings += tag.RowsAffected()

		if _, err := pool.Exec(ctx, `
			INSERT INTO inventory (listing_id, quantity)
			SELECT * FROM unnest($1::uuid[], $2::int[])
			ON CONFLICT (listing_id) DO NOTHING
		`, ids, quantities); err != nil {
			return result, fmt.Errorf("insert inventory batch at %d: %w", offset, err)
		}
	}

	result.Inserted = result.Listings
	result.Duration = time.Since(started)
	return result, nil
}
