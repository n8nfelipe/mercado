package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"mercado/internal/storage"
)

type memoryBlobStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
}

func newMemoryBlobStore() *memoryBlobStore {
	return &memoryBlobStore{objects: make(map[string][]byte), types: make(map[string]string)}
}

func (m *memoryBlobStore) Put(_ context.Context, key string, reader io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		panic("size mismatch")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = data
	m.types[key] = contentType
	return nil
}

func (m *memoryBlobStore) Get(_ context.Context, key string) (io.ReadCloser, int64, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, 0, "", storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), m.types[key], nil
}

func (m *memoryBlobStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	delete(m.types, key)
	return nil
}

func (m *memoryBlobStore) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok
}

func uploadImageRequest(t *testing.T, path string, cookie UserCookie, data []byte, _ string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "photo")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://web.test")
	request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	return request
}

func TestImagesInventoryAndMedia(t *testing.T) {
	blobs := newMemoryBlobStore()
	server := authTestServer(t, func(deps *Deps) { deps.Storage = blobs })

	adminCookie, admin := registerAndLogin(t, server, "images-admin@example.com")
	if err := server.store.AddRole(context.Background(), admin.ID, "admin"); err != nil {
		t.Fatalf("add admin role: %v", err)
	}
	response := perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/admin/categories", map[string]any{
		"name": "Fotos", "slug": "fotos",
	}, adminCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("create category = %d: %s", response.Code, response.Body.String())
	}
	var category categoryResponse
	_ = json.Unmarshal(response.Body.Bytes(), &category)

	sellerCookie, _ := registerAndLogin(t, server, "images-seller@example.com")
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/onboarding", map[string]any{
		"display_name": "Loja Fotos", "document": "123.456.789-00", "contact": "images-seller@example.com",
	}, sellerCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("onboarding = %d: %s", response.Code, response.Body.String())
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings", map[string]any{
		"title": "Câmera analógica revisada", "description": "Câmera em bom estado com lente original.",
		"category_id": category.ID.String(), "price_cents": 50000, "currency": "BRL",
		"condition": "used", "free_shipping": false, "stock": 2,
	}, sellerCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("create listing = %d: %s", response.Code, response.Body.String())
	}
	var listing sellerListingResponse
	_ = json.Unmarshal(response.Body.Bytes(), &listing)
	imagePath := "/v1/seller/listings/" + listing.ID.String() + "/images"

	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	response = perform(server.Handler(), uploadImageRequest(t, imagePath, sellerCookie, png, "image/png"))
	if response.Code != http.StatusCreated {
		t.Fatalf("upload image = %d: %s", response.Code, response.Body.String())
	}
	var first listingImageResponse
	_ = json.Unmarshal(response.Body.Bytes(), &first)
	if first.Position != 0 || first.URL == "" {
		t.Fatalf("unexpected image response: %+v", first)
	}

	media := perform(server.Handler(), httptest.NewRequest(http.MethodGet, first.URL, nil))
	if media.Code != http.StatusOK || media.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("media = %d %q: %s", media.Code, media.Header().Get("Content-Type"), media.Body.String())
	}
	if !bytes.Equal(media.Body.Bytes(), png) {
		t.Fatal("media bytes differ from upload")
	}

	response = perform(server.Handler(), uploadImageRequest(t, imagePath, sellerCookie, append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 32)...), "image/jpeg"))
	if response.Code != http.StatusCreated {
		t.Fatalf("upload second image = %d: %s", response.Code, response.Body.String())
	}
	var second listingImageResponse
	_ = json.Unmarshal(response.Body.Bytes(), &second)

	response = perform(server.Handler(), requestJSON(t, http.MethodPatch, imagePath+"/order", map[string]any{
		"image_ids": []string{second.ID.String(), first.ID.String()},
	}, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("reorder images = %d: %s", response.Code, response.Body.String())
	}
	var reordered []listingImageResponse
	_ = json.Unmarshal(response.Body.Bytes(), &reordered)
	if len(reordered) != 2 || reordered[0].ID != second.ID || reordered[0].Position != 0 || reordered[1].Position != 1 {
		t.Fatalf("unexpected image order: %+v", reordered)
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+listing.ID.String()+"/inventory", map[string]any{
		"delta": 3, "reason": "reposição de estoque",
	}, sellerCookie))
	if response.Code != http.StatusOK {
		t.Fatalf("adjust inventory = %d: %s", response.Code, response.Body.String())
	}
	var adjustment inventoryAdjustmentResponse
	_ = json.Unmarshal(response.Body.Bytes(), &adjustment)
	if adjustment.Quantity != 5 || adjustment.Delta != 3 {
		t.Fatalf("unexpected adjustment: %+v", adjustment)
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/listings/"+listing.ID.String()+"/inventory", map[string]any{
		"delta": -100, "reason": "quebra de estoque",
	}, sellerCookie))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "insufficient_stock" {
		t.Fatalf("negative stock = %d: %s", response.Code, response.Body.String())
	}
	response = perform(server.Handler(), requestJSON(t, http.MethodGet, "/v1/seller/listings/"+listing.ID.String()+"/inventory", nil, sellerCookie))
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("reposição")) {
		t.Fatalf("inventory history = %d: %s", response.Code, response.Body.String())
	}

	response = perform(server.Handler(), requestJSON(t, http.MethodDelete, imagePath+"/"+first.ID.String(), nil, sellerCookie))
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete image = %d: %s", response.Code, response.Body.String())
	}
	media = perform(server.Handler(), httptest.NewRequest(http.MethodGet, first.URL, nil))
	if media.Code != http.StatusNotFound {
		t.Fatalf("deleted media = %d: %s", media.Code, media.Body.String())
	}
	if blobs.has("listings/" + listing.ID.String() + "/" + first.ID.String() + ".png") {
		t.Fatal("image object was not deleted")
	}

	response = perform(server.Handler(), uploadImageRequest(t, imagePath, sellerCookie, []byte("not an image"), "image/png"))
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid image type = %d: %s", response.Code, response.Body.String())
	}
	response = perform(server.Handler(), uploadImageRequest(t, imagePath, sellerCookie, append(png, bytes.Repeat([]byte{0}, maxImageBytes)...), "image/png"))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized image = %d: %s", response.Code, response.Body.String())
	}

	for i := 0; i < 7; i++ {
		response = perform(server.Handler(), uploadImageRequest(t, imagePath, sellerCookie, png, "image/png"))
		if response.Code != http.StatusCreated {
			t.Fatalf("upload image %d = %d: %s", i, response.Code, response.Body.String())
		}
	}
	response = perform(server.Handler(), uploadImageRequest(t, imagePath, sellerCookie, png, "image/png"))
	if response.Code != http.StatusConflict || decodeError(t, response).Error.Code != "image_limit_reached" {
		t.Fatalf("image limit = %d: %s", response.Code, response.Body.String())
	}

	otherCookie, _ := registerAndLogin(t, server, "images-other@example.com")
	response = perform(server.Handler(), requestJSON(t, http.MethodPost, "/v1/seller/onboarding", map[string]any{
		"display_name": "Outra Loja Fotos", "document": "999.888.777-66", "contact": "images-other@example.com",
	}, otherCookie))
	if response.Code != http.StatusCreated {
		t.Fatalf("other onboarding = %d: %s", response.Code, response.Body.String())
	}
	response = perform(server.Handler(), uploadImageRequest(t, imagePath, otherCookie, png, "image/png"))
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign upload = %d: %s", response.Code, response.Body.String())
	}
}
