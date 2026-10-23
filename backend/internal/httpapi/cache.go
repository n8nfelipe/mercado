package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"mercado/internal/store"
)

const (
	categoriesCacheKey   = "catalog:categories:v1"
	categoriesCacheTTL   = 60 * time.Second
	listingTotalCacheTTL = 30 * time.Second
)

func (s *Server) cachedCategories(ctx context.Context) ([]store.Category, bool) {
	if s.cache == nil {
		return nil, false
	}
	raw, ok := s.cache.Get(ctx, categoriesCacheKey)
	if !ok {
		return nil, false
	}
	var categories []store.Category
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		s.cache.Delete(ctx, categoriesCacheKey)
		return nil, false
	}
	return categories, true
}

func (s *Server) cacheCategories(ctx context.Context, categories []store.Category) {
	if s.cache == nil {
		return
	}
	raw, err := json.Marshal(categories)
	if err != nil {
		return
	}
	s.cache.Set(ctx, categoriesCacheKey, string(raw), categoriesCacheTTL)
}

func (s *Server) invalidateCategories(ctx context.Context) {
	if s.cache == nil {
		return
	}
	s.cache.Delete(ctx, categoriesCacheKey)
}

func listingTotalCacheKey(params store.ListingSearch) string {
	var builder strings.Builder
	builder.WriteString("q=")
	builder.WriteString(params.Query)
	builder.WriteString("\x00category=")
	if params.CategoryID != nil {
		builder.WriteString(params.CategoryID.String())
	}
	builder.WriteString("\x00min=")
	if params.MinPriceCents != nil {
		builder.WriteString(strconv.FormatInt(*params.MinPriceCents, 10))
	}
	builder.WriteString("\x00max=")
	if params.MaxPriceCents != nil {
		builder.WriteString(strconv.FormatInt(*params.MaxPriceCents, 10))
	}
	builder.WriteString("\x00condition=")
	builder.WriteString(params.Condition)
	builder.WriteString("\x00free_shipping=")
	if params.FreeShipping != nil {
		builder.WriteString(strconv.FormatBool(*params.FreeShipping))
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "catalog:total:v1:" + hex.EncodeToString(sum[:])
}
