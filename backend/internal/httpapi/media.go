package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mercado/internal/storage"
	"mercado/internal/store"
)

type BlobStore interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
	Delete(ctx context.Context, key string) error
}

const maxImageBytes = 5 << 20

var mediaFileNamePattern = regexp.MustCompile(`^[a-f0-9-]+\.(jpg|jpeg|png|webp)$`)

type imageOrderRequest struct {
	ImageIDs []string `json:"image_ids"`
}

func detectImageType(data []byte) (string, string, bool) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "image/jpeg", "jpg", true
	case "image/png":
		return "image/png", "png", true
	case "image/webp":
		return "image/webp", "webp", true
	default:
		return "", "", false
	}
}

func (s *Server) uploadListingImage(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.storage == nil {
		writeError(w, r, http.StatusServiceUnavailable, "storage_unavailable", "image storage is unavailable")
		return
	}
	if err := s.store.CanAddListingImage(r.Context(), listingID, profile.ID); err != nil {
		s.writeImageError(w, r, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+512<<10)
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds the 5 MiB limit")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeValidationError(w, r, []FieldError{{Field: "file", Message: "file is required"}})
		return
	}
	defer file.Close()

	if header.Size > maxImageBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds the 5 MiB limit")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_image", "could not read image")
		return
	}
	if len(data) > maxImageBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds the 5 MiB limit")
		return
	}
	contentType, extension, ok := detectImageType(data)
	if !ok {
		writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_image_type", "only JPEG, PNG and WebP images are supported")
		return
	}

	imageID, err := uuid.NewV7()
	if err != nil {
		s.internalError(w, r, "new image id", err)
		return
	}
	objectKey := path.Join("listings", listingID.String(), imageID.String()+"."+extension)
	if err := s.storage.Put(r.Context(), objectKey, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		s.internalError(w, r, "upload image", err)
		return
	}
	if err := s.store.AddListingImage(r.Context(), listingID, profile.ID, imageID, objectKey, contentType, int64(len(data))); err != nil {
		if deleteErr := s.storage.Delete(r.Context(), objectKey); deleteErr != nil {
			s.logger.Warn("cleanup orphan image", "error", deleteErr)
		}
		s.writeImageError(w, r, err)
		return
	}

	listing, err := s.store.ListingByID(r.Context(), listingID)
	if err != nil {
		s.internalError(w, r, "select listing after image upload", err)
		return
	}
	for _, image := range listing.Images {
		if image.ID == imageID {
			writeJSON(w, http.StatusCreated, listingImageResponse{ID: image.ID, URL: image.URL, Position: image.Position})
			return
		}
	}
	s.internalError(w, r, "find uploaded image", errors.New("image metadata not found"))
}

func (s *Server) reorderListingImages(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	var request imageOrderRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.ImageIDs) == 0 || len(request.ImageIDs) > 8 {
		writeValidationError(w, r, []FieldError{{Field: "image_ids", Message: "image_ids must have between 1 and 8 items"}})
		return
	}
	ids := make([]uuid.UUID, 0, len(request.ImageIDs))
	for _, raw := range request.ImageIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			writeValidationError(w, r, []FieldError{{Field: "image_ids", Message: "image_ids must contain UUIDs"}})
			return
		}
		ids = append(ids, id)
	}
	listing, err := s.store.ReorderListingImages(r.Context(), listingID, profile.ID, ids)
	if err != nil {
		s.writeImageError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toListingImages(listing.Images))
}

func (s *Server) deleteListingImage(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	imageID, err := uuid.Parse(chi.URLParam(r, "imageId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
		return
	}
	objectKey, err := s.store.DeleteListingImage(r.Context(), listingID, profile.ID, imageID)
	if err != nil {
		s.writeImageError(w, r, err)
		return
	}
	if s.storage != nil {
		if err := s.storage.Delete(r.Context(), objectKey); err != nil {
			s.logger.Warn("delete image object", "correlation_id", CorrelationID(r), "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) serveListingMedia(w http.ResponseWriter, r *http.Request) {
	listingID, err := uuid.Parse(chi.URLParam(r, "listingId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
		return
	}
	fileName := chi.URLParam(r, "fileName")
	if !mediaFileNamePattern.MatchString(fileName) {
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
		return
	}
	objectKey := path.Join("listings", listingID.String(), fileName)
	contentType, err := s.store.ImageContentTypeByObjectKey(r.Context(), objectKey)
	if errors.Is(err, store.ErrImageNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
		return
	}
	if err != nil {
		s.internalError(w, r, "select image metadata", err)
		return
	}
	if s.storage == nil {
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
		return
	}

	reader, size, _, err := s.storage.Get(r.Context(), objectKey)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
		return
	}
	if err != nil {
		s.internalError(w, r, "read image object", err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func (s *Server) writeImageError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrListingNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "listing not found")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, store.ErrImageNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "image not found")
	case errors.Is(err, store.ErrInvalidState):
		writeError(w, r, http.StatusConflict, "invalid_listing_state", "listing state does not allow this operation")
	case errors.Is(err, store.ErrImageLimit):
		writeError(w, r, http.StatusConflict, "image_limit_reached", "listing already has 8 images")
	case errors.Is(err, store.ErrInvalidListingData):
		writeError(w, r, http.StatusBadRequest, "invalid_image_order", "image_ids must match the listing images")
	case err != nil:
		s.internalError(w, r, "image operation", err)
	}
}
