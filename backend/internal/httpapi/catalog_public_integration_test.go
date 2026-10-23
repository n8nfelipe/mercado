package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPublicCatalogSearchAndDetail(t *testing.T) {
	server := authTestServer(t, nil)

	adminCookie, admin := registerAndLogin(t, server, "public-admin@example.com")
	if err := server.store.AddRole(context.Background(), admin.ID, "admin"); err != nil {
		t.Fatalf("add admin role: %v", err)
	}
	response := perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
		"name": "Bicicletas", "slug": "bicicletas",
	}, adminCookie))
	var bikes categoryResponse
	_ = json.Unmarshal(response.Body.Bytes(), &bikes)
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
		"name": "Acessórios", "slug": "acessorios",
	}, adminCookie))
	var accessories categoryResponse
	_ = json.Unmarshal(response.Body.Bytes(), &accessories)

	sellerCookie, _ := registerAndLogin(t, server, "public-seller@example.com")
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/onboarding", map[string]any{
		"display_name": "Loja Pública", "document": "111.222.333-44", "contact": "contato-privado@example.com",
	}, sellerCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("onboarding = %d: %s", response.Code, response.Body.String())
	}

	create := func(title string, category categoryResponse, price int64, condition string, free bool) sellerListingResponse {
		t.Helper()
		response := perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings", map[string]any{
			"title": title, "description": "Descrição detalhada do anúncio público de teste.",
			"category_id": category.ID.String(), "price_cents": price, "currency": "BRL",
			"condition": condition, "free_shipping": free, "stock": 2,
		}, sellerCookie))
		if response.Code != http.StatusCreated {
			t.Fatalf("create listing = %d: %s", response.Code, response.Body.String())
		}
		var listing sellerListingResponse
		_ = json.Unmarshal(response.Body.Bytes(), &listing)
		return listing
	}
	publish := func(listing sellerListingResponse) {
		t.Helper()
		response := perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+listing.ID.String()+"/submit", nil, sellerCookie))
		if response.Code != http.StatusOK {
			t.Fatalf("submit = %d: %s", response.Code, response.Body.String())
		}
		response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/listings/"+listing.ID.String()+"/publish", nil, adminCookie))
		if response.Code != http.StatusOK {
			t.Fatalf("publish = %d: %s", response.Code, response.Body.String())
		}
	}

	bike := create("Bicicleta Aro 29 Publicada", bikes, 100000, "used", true)
	helmet := create("Capacete Ótimo Estado Novo", accessories, 5000, "new", false)
	draft := create("Rascunho Oculto no Catálogo", bikes, 9000, "used", false)
	publish(bike)
	publish(helmet)

	search := func(query string) listingPageResponse {
		t.Helper()
		response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/catalog/listings?"+query, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("search %q = %d: %s", query, response.Code, response.Body.String())
		}
		var page listingPageResponse
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode search: %v", err)
		}
		return page
	}

	all := search("page=1&page_size=20")
	if all.Total != 2 {
		t.Fatalf("public listings = %d, want 2: %+v", all.Total, all.Items)
	}
	for _, item := range all.Items {
		if item.ID == draft.ID {
			t.Fatal("draft listing is publicly listed")
		}
		if item.CoverImageURL != nil {
			t.Fatal("listing without images should not expose a cover")
		}
	}

	normalized := search("q=" + url.QueryEscape("otimo"))
	if normalized.Total != 1 || normalized.Items[0].ID != helmet.ID {
		t.Fatalf("accent-insensitive public search failed: %+v", normalized.Items)
	}
	byCategory := search("category_id=" + accessories.ID.String())
	if byCategory.Total != 1 || byCategory.Items[0].ID != helmet.ID {
		t.Fatalf("public category filter failed: %+v", byCategory.Items)
	}
	if filtered := search("condition=new"); filtered.Total != 1 {
		t.Fatalf("public condition filter failed: %+v", filtered.Items)
	}
	if filtered := search("free_shipping=true"); filtered.Total != 1 || filtered.Items[0].ID != bike.ID {
		t.Fatalf("public shipping filter failed: %+v", filtered.Items)
	}
	if filtered := search("min_price_cents=50000"); filtered.Total != 1 || filtered.Items[0].ID != bike.ID {
		t.Fatalf("public price filter failed: %+v", filtered.Items)
	}
	ordered := search("sort=price_asc")
	if len(ordered.Items) != 2 || ordered.Items[0].PriceCents != 5000 {
		t.Fatalf("public price order failed: %+v", ordered.Items)
	}
	paged := search("sort=price_asc&page=2&page_size=1")
	if len(paged.Items) != 1 || paged.Page != 2 || paged.TotalPages != 2 {
		t.Fatalf("public pagination failed: %+v", paged)
	}
	for _, invalid := range []string{"page=0", "page_size=51", "sort=random", "condition=broken", "min_price_cents=abc", "category_id=nope"} {
		response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/catalog/listings?"+invalid, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid query %q = %d: %s", invalid, response.Code, response.Body.String())
		}
	}

	response = perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/listings/"+bike.ID.String(), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("public detail = %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, private := range []string{"111.222.333-44", "contato-privado@example.com", "\"document\"", "\"contact\""} {
		if strings.Contains(body, private) {
			t.Fatalf("public detail leaked %q: %s", private, body)
		}
	}
	var detail listingDetailResponse
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil || detail.Seller.DisplayName != "Loja Pública" {
		t.Fatalf("unexpected public detail: %s", body)
	}

	for _, hidden := range []string{draft.ID.String(), "00000000-0000-0000-0000-000000000000"} {
		response = perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/listings/"+hidden, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("hidden detail %s = %d: %s", hidden, response.Code, response.Body.String())
		}
	}

	if empty := search("q=inexistente-sem-resultado"); empty.Total != 0 {
		t.Fatalf("empty search returned results: %+v", empty.Items)
	}

	metrics := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, series := range []string{
		"catalog_searches_total{result=\"empty\"}",
		"catalog_searches_total{result=\"results\"}",
		"catalog_search_filters_total{filter=\"q\"}",
		"catalog_search_filters_total{filter=\"category_id\"}",
		"catalog_listing_not_found_total 2",
	} {
		if !strings.Contains(metrics.Body.String(), series) {
			t.Fatalf("metrics missing %q:\n%s", series, metrics.Body.String())
		}
	}
	if !strings.Contains(metrics.Body.String(), "catalog_searches_total{result=\"empty\"} 1\n") {
		t.Fatalf("expected exactly one empty search:\n%s", metrics.Body.String())
	}

	// A suspended listing disappears from the public catalog immediately.
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/listings/"+helmet.ID.String()+"/suspend", map[string]any{
		"reason": "Anúncio suspenso para revisão",
	}, adminCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("suspend = %d: %s", response.Code, response.Body.String())
	}
	if after := search("page=1&page_size=20"); after.Total != 1 || after.Items[0].ID != bike.ID {
		t.Fatalf("suspended listing still public: %+v", after.Items)
	}
	response = perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/listings/"+helmet.ID.String(), nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("suspended detail = %d: %s", response.Code, response.Body.String())
	}
}
