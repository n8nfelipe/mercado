package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"mercado/internal/store"
)

func (s *Server) listPublicListings(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	params := store.ListingSearch{
		Query:    strings.TrimSpace(query.Get("q")),
		Sort:     strings.TrimSpace(query.Get("sort")),
		Page:     1,
		PageSize: 20,
	}
	details := make([]FieldError, 0)
	if len([]rune(params.Query)) > 200 {
		details = append(details, FieldError{Field: "q", Message: "q must have at most 200 characters"})
	}
	if raw := query.Get("category_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			details = append(details, FieldError{Field: "category_id", Message: "category_id must be a UUID"})
		} else {
			params.CategoryID = &id
		}
	}
	if params.MinPriceCents, details = parseOptionalInt64(query.Get("min_price_cents"), "min_price_cents", details); params.MinPriceCents != nil && *params.MinPriceCents < 0 {
		details = append(details, FieldError{Field: "min_price_cents", Message: "min_price_cents must be zero or positive"})
	}
	if params.MaxPriceCents, details = parseOptionalInt64(query.Get("max_price_cents"), "max_price_cents", details); params.MaxPriceCents != nil && *params.MaxPriceCents < 0 {
		details = append(details, FieldError{Field: "max_price_cents", Message: "max_price_cents must be zero or positive"})
	}
	if params.MinPriceCents != nil && params.MaxPriceCents != nil && *params.MinPriceCents > *params.MaxPriceCents {
		details = append(details, FieldError{Field: "max_price_cents", Message: "max_price_cents must be greater than or equal to min_price_cents"})
	}
	if raw := query.Get("condition"); raw != "" {
		if raw != "new" && raw != "used" {
			details = append(details, FieldError{Field: "condition", Message: "condition must be new or used"})
		} else {
			params.Condition = raw
		}
	}
	if raw := query.Get("free_shipping"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			details = append(details, FieldError{Field: "free_shipping", Message: "free_shipping must be a boolean"})
		} else {
			params.FreeShipping = &value
		}
	}
	if params.Sort != "" && params.Sort != "relevance" && params.Sort != "price_asc" && params.Sort != "price_desc" && params.Sort != "newest" {
		details = append(details, FieldError{Field: "sort", Message: "sort must be relevance, price_asc, price_desc or newest"})
	}
	if raw := query.Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			details = append(details, FieldError{Field: "page", Message: "page must be a positive integer"})
		} else {
			params.Page = value
		}
	}
	if raw := query.Get("page_size"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 50 {
			details = append(details, FieldError{Field: "page_size", Message: "page_size must be between 1 and 50"})
		} else {
			params.PageSize = value
		}
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return
	}

	totalKey := listingTotalCacheKey(params)
	cachedTotal := false
	if s.cache != nil {
		if raw, ok := s.cache.Get(r.Context(), totalKey); ok {
			if total, err := strconv.Atoi(raw); err == nil {
				params.KnownTotal = &total
				cachedTotal = true
			}
		}
	}

	page, err := s.store.SearchListings(r.Context(), params)
	if err != nil {
		s.internalError(w, r, "search listings", err)
		return
	}
	if s.cache != nil && !cachedTotal {
		s.cache.Set(r.Context(), totalKey, strconv.Itoa(page.Total), listingTotalCacheTTL)
	}
	for _, filter := range []struct {
		name  string
		apply bool
	}{
		{"q", params.Query != ""},
		{"category_id", params.CategoryID != nil},
		{"min_price_cents", params.MinPriceCents != nil},
		{"max_price_cents", params.MaxPriceCents != nil},
		{"condition", params.Condition != ""},
		{"free_shipping", params.FreeShipping != nil},
		{"sort", params.Sort != "" && params.Sort != "relevance"},
	} {
		if filter.apply {
			s.metrics.catalogSearchFilters.WithLabelValues(filter.name).Inc()
		}
	}
	result := "results"
	if page.Total == 0 {
		result = "empty"
	}
	s.metrics.catalogSearches.WithLabelValues(result).Inc()

	items := make([]listingSummaryResponse, 0, len(page.Items))
	for _, listing := range page.Items {
		items = append(items, toListingSummary(listing))
	}
	writeJSON(w, http.StatusOK, listingPageResponse{
		Items:      items,
		Page:       page.Page,
		PageSize:   page.PageSize,
		Total:      page.Total,
		TotalPages: page.TotalPages,
	})
}

func (s *Server) getPublicListing(w http.ResponseWriter, r *http.Request) {
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	listing, err := s.store.PublicListingByID(r.Context(), listingID)
	if errors.Is(err, store.ErrListingNotFound) {
		s.metrics.catalogListingNotFound.Inc()
		writeError(w, r, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		s.internalError(w, r, "select public listing", err)
		return
	}
	writeJSON(w, http.StatusOK, toListingDetail(listing))
}

func parseOptionalInt64(raw, field string, details []FieldError) (*int64, []FieldError) {
	if raw == "" {
		return nil, details
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, append(details, FieldError{Field: field, Message: field + " must be an integer"})
	}
	return &value, details
}
