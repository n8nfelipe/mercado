package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func requestJSON(t *testing.T, method, path string, body any, cookie UserCookie) *http.Request {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, &payload)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://web.test")
	if cookie.Name != "" {
		request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return request
}

func TestCategoriesListingsAndModeration(t *testing.T) {
	server := authTestServer(t, nil)

	adminCookie, admin := registerAndLogin(t, server, "admin@example.com")
	if err := server.store.AddRole(context.Background(), admin.ID, "admin"); err != nil {
		t.Fatalf("add admin role: %v", err)
	}

	response := perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
		"name": "Bicicletas", "slug": "bicicletas",
	}, adminCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("create category = %d: %s", response.Code, response.Body.String())
	}
	var category categoryResponse
	if err := json.Unmarshal(response.Body.Bytes(), &category); err != nil {
		t.Fatalf("decode category: %v", err)
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
		"name": "Outra", "slug": "bicicletas",
	}, adminCookie))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "category_slug_taken" {
		t.Fatalf("duplicate slug = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
		"name": "Acessórios", "slug": "acessorios",
	}, adminCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("create second category = %d: %s", response.Code, response.Body.String())
	}
	var secondCategory categoryResponse
	_ = json.Unmarshal(response.Body.Bytes(), &secondCategory)

	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/admin/categories/"+secondCategory.ID.String(), map[string]any{
		"name": "Acessórios", "slug": "bicicletas",
	}, adminCookie))
	if response.Code != http.StatusConflict {
		t.Fatalf("rename category to duplicate slug = %d: %s", response.Code, response.Body.String())
	}

	// Categories are public only while active.
	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/admin/categories/"+category.ID.String(), map[string]any{
		"name": "Bicicletas", "slug": "bicicletas", "is_active": false,
	}, adminCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("deactivate category = %d: %s", response.Code, response.Body.String())
	}
	public := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/categories", nil))
	if strings.Contains(public.Body.String(), "bicicletas") {
		t.Fatalf("inactive category is public: %s", public.Body.String())
	}

	sellerCookie, _ := registerAndLogin(t, server, "seller@example.com")
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/onboarding", map[string]any{
		"display_name": "Loja do Vendedor", "document": "111.222.333-44", "contact": "seller@example.com",
	}, sellerCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("onboarding = %d: %s", response.Code, response.Body.String())
	}

	listingBody := map[string]any{
		"title": "Bicicleta Aro 29 Usada", "description": "Bicicleta em ótimo estado, revisada recentemente.",
		"category_id": category.ID.String(), "price_cents": 120000, "currency": "BRL",
		"condition": "used", "free_shipping": true, "stock": 2,
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings", listingBody, sellerCookie))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("listing with inactive category = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/admin/categories/"+category.ID.String(), map[string]any{
		"name": "Bicicletas", "slug": "bicicletas", "is_active": true,
	}, adminCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("reactivate category = %d: %s", response.Code, response.Body.String())
	}

	invalid := map[string]any{}
	for key, value := range listingBody {
		invalid[key] = value
	}
	invalid["title"] = "curto"
	invalid["condition"] = "broken"
	invalid["price_cents"] = 0
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings", invalid, sellerCookie))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid listing = %d: %s", response.Code, response.Body.String())
	}
	body := decodeError(t, response)
	if len(body.Error.Details) < 3 {
		t.Fatalf("expected multiple field errors, got %s", response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings", listingBody, sellerCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("create listing = %d: %s", response.Code, response.Body.String())
	}
	var listing sellerListingResponse
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil {
		t.Fatalf("decode listing: %v", err)
	}
	if listing.Status != "draft" {
		t.Fatalf("new listing status = %s, want draft", listing.Status)
	}

	// A second seller cannot read or modify the first seller's listing.
	otherCookie, _ := registerAndLogin(t, server, "other@example.com")
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/onboarding", map[string]any{
		"display_name": "Outra Loja", "document": "555.666.777-88", "contact": "other@example.com",
	}, otherCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("second onboarding = %d: %s", response.Code, response.Body.String())
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodGet, "/v1/seller/listings/"+listing.ID.String(), nil, otherCookie))
	if response.Code != http.StatusForbidden {
		t.Fatalf("other seller read = %d, want 403", response.Code)
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/seller/listings/"+listing.ID.String(), map[string]any{"title": "Título de outro vendedor"}, otherCookie))
	if response.Code != http.StatusForbidden {
		t.Fatalf("other seller edit = %d, want 403", response.Code)
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/seller/listings/"+listing.ID.String(), map[string]any{
		"title": "Bicicleta Aro 29 Revisada",
	}, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("edit draft = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+listing.ID.String()+"/submit", nil, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("submit = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil || listing.Status != "pending_review" {
		t.Fatalf("submitted listing = %s (%v)", response.Body.String(), err)
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodGet, "/v1/admin/listings/pending", nil, adminCookie))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), listing.ID.String()) {
		t.Fatalf("pending queue = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/listings/"+listing.ID.String()+"/publish", nil, adminCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("publish = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil || listing.Status != "published" {
		t.Fatalf("published listing = %s (%v)", response.Body.String(), err)
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/seller/listings/"+listing.ID.String(), map[string]any{
		"title": "Título novo sem revisão",
	}, sellerCookie))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "listing_requires_review" {
		t.Fatalf("content edit published = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, "/v1/seller/listings/"+listing.ID.String(), map[string]any{
		"price_cents": 110000, "stock": 5,
	}, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("price and stock edit = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil || listing.PriceCents != 110000 || listing.Stock != 5 {
		t.Fatalf("price/stock not updated: %s", response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/listings/"+listing.ID.String()+"/suspend", map[string]any{
		"reason": "Anúncio fora das regras",
	}, adminCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("suspend = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil || listing.Status != "suspended" || listing.RejectionReason == nil {
		t.Fatalf("suspended listing = %s", response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+listing.ID.String()+"/archive", nil, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("archive = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil || listing.Status != "archived" {
		t.Fatalf("archived listing = %s", response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+listing.ID.String()+"/submit", nil, sellerCookie))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "invalid_listing_state" {
		t.Fatalf("submit archived = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodDelete, "/v1/admin/categories/"+category.ID.String(), nil, adminCookie))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "category_in_use" {
		t.Fatalf("delete category in use = %d: %s", response.Code, response.Body.String())
	}

	// Rejection requires a reason and lets the seller resubmit.
	listingBody["title"] = "Segunda Bicicleta Aro 29"
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings", listingBody, sellerCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("create second listing = %d: %s", response.Code, response.Body.String())
	}
	var rejected sellerListingResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rejected); err != nil {
		t.Fatalf("decode second listing: %v", err)
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+rejected.ID.String()+"/submit", nil, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("submit second listing = %d: %s", response.Code, response.Body.String())
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/listings/"+rejected.ID.String()+"/reject", map[string]any{
		"reason": "Fotos insuficientes",
	}, adminCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("reject = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rejected); err != nil || rejected.Status != "rejected" || rejected.RejectionReason == nil {
		t.Fatalf("rejected listing = %s", response.Body.String())
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+rejected.ID.String()+"/submit", nil, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("resubmit rejected = %d: %s", response.Code, response.Body.String())
	}

	metrics := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, series := range []string{
		"listings_by_status{status=\"archived\"} 1",
		"listing_moderation_decisions_total{action=\"publish\"} 1",
		"listing_moderation_decisions_total{action=\"suspend\"} 1",
	} {
		if !strings.Contains(metrics.Body.String(), series) {
			t.Fatalf("metrics missing %q:\n%s", series, metrics.Body.String())
		}
	}

}
