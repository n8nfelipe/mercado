package httpapi

import (
	"testing"

	"mercado/internal/store"
)

func TestListingTotalCacheKeyIgnoresPagination(t *testing.T) {
	base := listingTotalCacheKey(store.ListingSearch{Query: "capacete"})
	same := listingTotalCacheKey(store.ListingSearch{Query: "capacete", Sort: "price_desc", Page: 3, PageSize: 50})
	if base != same {
		t.Fatal("pagination and sort must not change the total cache key")
	}
}

func TestListingTotalCacheKeyDistinguishesFilters(t *testing.T) {
	free := false
	base := listingTotalCacheKey(store.ListingSearch{})
	variants := []store.ListingSearch{
		{Query: "capacete"},
		{Condition: "new"},
		{FreeShipping: &free},
		{MinPriceCents: ptrInt64(100)},
		{MaxPriceCents: ptrInt64(100)},
	}
	for _, variant := range variants {
		if listingTotalCacheKey(variant) == base {
			t.Fatalf("filters %+v must produce a distinct total cache key", variant)
		}
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}
