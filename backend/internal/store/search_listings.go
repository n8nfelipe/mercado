package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"mercado/internal/search"
)

type ListingSearch struct {
	Query         string
	CategoryID    *uuid.UUID
	MinPriceCents *int64
	MaxPriceCents *int64
	Condition     string
	FreeShipping  *bool
	Sort          string
	Page          int
	PageSize      int
	KnownTotal    *int
}

type ListingPage struct {
	Items      []Listing
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

func (s *Store) SearchListings(ctx context.Context, params ListingSearch) (ListingPage, error) {
	conditions := []string{"l.status = 'published'", "c.is_active"}
	args := make([]any, 0)
	nextArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	query := search.Normalize(params.Query)
	if query != "" {
		conditions = append(conditions, "l.search_text LIKE '%' || "+nextArg(query)+" || '%'")
	}
	if params.CategoryID != nil {
		conditions = append(conditions, "l.category_id = "+nextArg(*params.CategoryID))
	}
	if params.MinPriceCents != nil {
		conditions = append(conditions, "l.price_cents >= "+nextArg(*params.MinPriceCents))
	}
	if params.MaxPriceCents != nil {
		conditions = append(conditions, "l.price_cents <= "+nextArg(*params.MaxPriceCents))
	}
	if params.Condition != "" {
		conditions = append(conditions, "l.condition = "+nextArg(params.Condition))
	}
	if params.FreeShipping != nil {
		conditions = append(conditions, "l.free_shipping = "+nextArg(*params.FreeShipping))
	}
	where := " WHERE " + strings.Join(conditions, " AND ")

	total := 0
	if params.KnownTotal != nil {
		total = *params.KnownTotal
	} else {
		countQuery := "SELECT count(*) FROM listings l JOIN categories c ON c.id = l.category_id" + where
		if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return ListingPage{}, fmt.Errorf("count listings: %w", err)
		}
	}

	order := " ORDER BY "
	switch params.Sort {
	case "price_asc":
		order += "l.price_cents ASC, l.id ASC"
	case "price_desc":
		order += "l.price_cents DESC, l.id DESC"
	case "newest":
		order += "l.published_at DESC, l.id DESC"
	default:
		if query != "" {
			order += "similarity(l.search_text, " + nextArg(query) + ") DESC, "
		}
		order += "l.published_at DESC, l.id DESC"
	}

	limit := nextArg(params.PageSize)
	offset := nextArg((params.Page - 1) * params.PageSize)
	rows, err := s.pool.Query(ctx, `
		SELECT `+listingColumns+`
		`+listingFrom+where+order+`
		LIMIT `+limit+` OFFSET `+offset,
		args...)
	if err != nil {
		return ListingPage{}, fmt.Errorf("search listings: %w", err)
	}
	listings, ids, err := scanListings(rows)
	if err != nil {
		return ListingPage{}, err
	}
	images, err := s.listingImages(ctx, ids)
	if err != nil {
		return ListingPage{}, err
	}
	for index := range listings {
		listings[index].Images = images[listings[index].ID]
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + params.PageSize - 1) / params.PageSize
	}
	return ListingPage{
		Items:      listings,
		Page:       params.Page,
		PageSize:   params.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}
