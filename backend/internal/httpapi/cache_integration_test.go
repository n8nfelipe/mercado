package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"mercado/internal/cache"
)

func TestPublicCatalogCachesCategoriesAndTotals(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	server := authTestServer(t, func(deps *Deps) {
		deps.Cache = cache.New(client, nil)
	})
	adminCookie, admin := registerAndLogin(t, server, "cache-admin@example.com")
	if err := server.store.AddRole(context.Background(), admin.ID, "admin"); err != nil {
		t.Fatalf("add admin role: %v", err)
	}

	categories := func() []categoryResponse {
		t.Helper()
		response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/categories", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("list categories = %d: %s", response.Code, response.Body.String())
		}
		var result []categoryResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode categories: %v", err)
		}
		return result
	}

	createCategory := func(name, slug string) {
		t.Helper()
		response := perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
			"name": name, "slug": slug,
		}, adminCookie))
		if response.Code != http.StatusCreated {
			t.Fatalf("create category = %d: %s", response.Code, response.Body.String())
		}
	}

	createCategory("Bicicletas", "bicicletas")
	if got := categories(); len(got) != 1 {
		t.Fatalf("categories = %d, want 1", len(got))
	}
	if !redisServer.Exists(categoriesCacheKey) {
		t.Fatal("categories were not cached after listing")
	}

	createCategory("Acessórios", "acessorios")
	if redisServer.Exists(categoriesCacheKey) {
		t.Fatal("category cache was not invalidated after create")
	}
	if got := categories(); len(got) != 2 {
		t.Fatalf("categories = %d, want 2", len(got))
	}
	if !redisServer.Exists(categoriesCacheKey) {
		t.Fatal("categories were not cached after invalidation")
	}

	response := perform(server.Handler(), httptest.NewRequest(http.MethodGet, "/v1/catalog/listings", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("list listings = %d: %s", response.Code, response.Body.String())
	}
	foundTotal := false
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, "catalog:total:v1:") {
			foundTotal = true
			break
		}
	}
	if !foundTotal {
		t.Fatal("listing total was not cached after search")
	}
}
