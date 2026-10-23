package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mercado/internal/store"
)

type categoryResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	ParentID  *uuid.UUID `json:"parent_id"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
}

type categoryRequest struct {
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	ParentID *string `json:"parent_id"`
	IsActive *bool   `json:"is_active"`
}

type listingImageResponse struct {
	ID       uuid.UUID `json:"id"`
	URL      string    `json:"url"`
	Position int       `json:"position"`
}

type shortCategoryResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type shortSellerResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
}

type listingSummaryResponse struct {
	ID            uuid.UUID             `json:"id"`
	Title         string                `json:"title"`
	PriceCents    int64                 `json:"price_cents"`
	Currency      string                `json:"currency"`
	Condition     string                `json:"condition"`
	FreeShipping  bool                  `json:"free_shipping"`
	Stock         int                   `json:"stock"`
	CoverImageURL *string               `json:"cover_image_url"`
	Category      shortCategoryResponse `json:"category"`
	Seller        shortSellerResponse   `json:"seller"`
	PublishedAt   time.Time             `json:"published_at"`
}

type listingDetailResponse struct {
	ID           uuid.UUID              `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	PriceCents   int64                  `json:"price_cents"`
	Currency     string                 `json:"currency"`
	Condition    string                 `json:"condition"`
	FreeShipping bool                   `json:"free_shipping"`
	Stock        int                    `json:"stock"`
	Category     shortCategoryResponse  `json:"category"`
	Seller       shortSellerResponse    `json:"seller"`
	Images       []listingImageResponse `json:"images"`
	PublishedAt  time.Time              `json:"published_at"`
}

type listingPageResponse struct {
	Items      []listingSummaryResponse `json:"items"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	Total      int                      `json:"total"`
	TotalPages int                      `json:"total_pages"`
}

type sellerListingResponse struct {
	ID              uuid.UUID              `json:"id"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	PriceCents      int64                  `json:"price_cents"`
	Currency        string                 `json:"currency"`
	Condition       string                 `json:"condition"`
	FreeShipping    bool                   `json:"free_shipping"`
	Stock           int                    `json:"stock"`
	Status          string                 `json:"status"`
	RejectionReason *string                `json:"rejection_reason"`
	Category        shortCategoryResponse  `json:"category"`
	Images          []listingImageResponse `json:"images"`
	SubmittedAt     *time.Time             `json:"submitted_at"`
	PublishedAt     *time.Time             `json:"published_at"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type createListingRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	CategoryID   string `json:"category_id"`
	PriceCents   *int64 `json:"price_cents"`
	Currency     string `json:"currency"`
	Condition    string `json:"condition"`
	FreeShipping *bool  `json:"free_shipping"`
	Stock        *int   `json:"stock"`
}

type updateListingRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	CategoryID   *string `json:"category_id"`
	PriceCents   *int64  `json:"price_cents"`
	Condition    *string `json:"condition"`
	FreeShipping *bool   `json:"free_shipping"`
	Stock        *int    `json:"stock"`
}

type moderationRequest struct {
	Reason string `json:"reason"`
}

type inventoryAdjustmentRequest struct {
	Delta  *int   `json:"delta"`
	Reason string `json:"reason"`
}

type inventoryAdjustmentResponse struct {
	ID        uuid.UUID `json:"id"`
	ListingID uuid.UUID `json:"listing_id"`
	ActorID   uuid.UUID `json:"actor_id"`
	Delta     int       `json:"delta"`
	Quantity  int       `json:"quantity"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func toCategoryResponse(category store.Category) categoryResponse {
	return categoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		Slug:      category.Slug,
		ParentID:  category.ParentID,
		IsActive:  category.IsActive,
		CreatedAt: category.CreatedAt,
	}
}

func toListingImages(images []store.ListingImage) []listingImageResponse {
	result := make([]listingImageResponse, 0, len(images))
	for _, image := range images {
		result = append(result, listingImageResponse{ID: image.ID, URL: image.URL, Position: image.Position})
	}
	return result
}

func toListingSummary(listing store.Listing) listingSummaryResponse {
	var cover *string
	if len(listing.Images) > 0 {
		url := listing.Images[0].URL
		cover = &url
	}
	publishedAt := time.Time{}
	if listing.PublishedAt != nil {
		publishedAt = *listing.PublishedAt
	}
	return listingSummaryResponse{
		ID:            listing.ID,
		Title:         listing.Title,
		PriceCents:    listing.PriceCents,
		Currency:      listing.Currency,
		Condition:     listing.Condition,
		FreeShipping:  listing.FreeShipping,
		Stock:         listing.Stock,
		CoverImageURL: cover,
		Category:      shortCategoryResponse{ID: listing.CategoryID, Name: listing.CategoryName, Slug: listing.CategorySlug},
		Seller:        shortSellerResponse{ID: listing.SellerID, DisplayName: listing.SellerName},
		PublishedAt:   publishedAt,
	}
}

func toListingDetail(listing store.Listing) listingDetailResponse {
	summary := toListingSummary(listing)
	images := toListingImages(listing.Images)
	return listingDetailResponse{
		ID:           summary.ID,
		Title:        summary.Title,
		Description:  listing.Description,
		PriceCents:   summary.PriceCents,
		Currency:     summary.Currency,
		Condition:    summary.Condition,
		FreeShipping: summary.FreeShipping,
		Stock:        summary.Stock,
		Category:     summary.Category,
		Seller:       summary.Seller,
		Images:       images,
		PublishedAt:  summary.PublishedAt,
	}
}

func toSellerListing(listing store.Listing) sellerListingResponse {
	return sellerListingResponse{
		ID:              listing.ID,
		Title:           listing.Title,
		Description:     listing.Description,
		PriceCents:      listing.PriceCents,
		Currency:        listing.Currency,
		Condition:       listing.Condition,
		FreeShipping:    listing.FreeShipping,
		Stock:           listing.Stock,
		Status:          listing.Status,
		RejectionReason: listing.RejectionReason,
		Category:        shortCategoryResponse{ID: listing.CategoryID, Name: listing.CategoryName, Slug: listing.CategorySlug},
		Images:          toListingImages(listing.Images),
		SubmittedAt:     listing.SubmittedAt,
		PublishedAt:     listing.PublishedAt,
		CreatedAt:       listing.CreatedAt,
		UpdatedAt:       listing.UpdatedAt,
	}
}

func toInventoryAdjustment(adjustment store.InventoryAdjustment) inventoryAdjustmentResponse {
	return inventoryAdjustmentResponse{
		ID:        adjustment.ID,
		ListingID: adjustment.ListingID,
		ActorID:   adjustment.ActorID,
		Delta:     adjustment.Delta,
		Quantity:  adjustment.Quantity,
		Reason:    adjustment.Reason,
		CreatedAt: adjustment.CreatedAt,
	}
}

func (s *Server) listPublicCategories(w http.ResponseWriter, r *http.Request) {
	if categories, ok := s.cachedCategories(r.Context()); ok {
		s.writeCategories(w, categories)
		return
	}
	categories, err := s.store.ListPublicCategories(r.Context())
	if err != nil {
		s.internalError(w, r, "list categories", err)
		return
	}
	s.cacheCategories(r.Context(), categories)
	s.writeCategories(w, categories)
}

func (s *Server) writeCategories(w http.ResponseWriter, categories []store.Category) {
	response := make([]categoryResponse, 0, len(categories))
	for _, category := range categories {
		response = append(response, toCategoryResponse(category))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	var request categoryRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	input, ok := validateCategory(w, r, request)
	if !ok {
		return
	}
	category, err := s.store.CreateCategory(r.Context(), input)
	s.writeCategoryError(w, r, category, err, http.StatusCreated)
}

func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "category not found")
		return
	}
	var request categoryRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	input, ok := validateCategory(w, r, request)
	if !ok {
		return
	}
	category, err := s.store.UpdateCategory(r.Context(), id, input)
	s.writeCategoryError(w, r, category, err, http.StatusOK)
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "category not found")
		return
	}
	err = s.store.DeleteCategory(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrCategoryNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "category not found")
	case errors.Is(err, store.ErrCategoryInUse):
		writeError(w, r, http.StatusConflict, "category_in_use", "category has listings or subcategories")
	case err != nil:
		s.internalError(w, r, "delete category", err)
	default:
		s.invalidateCategories(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) writeCategoryError(w http.ResponseWriter, r *http.Request, category store.Category, err error, status int) {
	switch {
	case errors.Is(err, store.ErrSlugTaken):
		writeError(w, r, http.StatusConflict, "category_slug_taken", "category slug already in use")
	case errors.Is(err, store.ErrCategoryParent):
		writeValidationError(w, r, []FieldError{{Field: "parent_id", Message: "parent category is invalid"}})
	case errors.Is(err, store.ErrCategoryNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "category not found")
	case err != nil:
		s.internalError(w, r, "write category", err)
	default:
		s.invalidateCategories(r.Context())
		writeJSON(w, status, toCategoryResponse(category))
	}
}

func validateCategory(w http.ResponseWriter, r *http.Request, request categoryRequest) (store.CategoryInput, bool) {
	name := strings.TrimSpace(request.Name)
	slug := strings.TrimSpace(request.Slug)
	details := make([]FieldError, 0)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		details = append(details, FieldError{Field: "name", Message: "name must have between 2 and 120 characters"})
	}
	if len(slug) < 2 || len(slug) > 120 || !slugPattern.MatchString(slug) {
		details = append(details, FieldError{Field: "slug", Message: "slug must use lowercase letters, numbers and hyphens"})
	}

	var parentID *uuid.UUID
	if request.ParentID != nil && strings.TrimSpace(*request.ParentID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.ParentID))
		if err != nil {
			details = append(details, FieldError{Field: "parent_id", Message: "parent_id must be a UUID"})
		} else {
			parentID = &parsed
		}
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return store.CategoryInput{}, false
	}
	active := true
	if request.IsActive != nil {
		active = *request.IsActive
	}
	return store.CategoryInput{Name: name, Slug: slug, ParentID: parentID, IsActive: active}, true
}

func validateListingCreate(w http.ResponseWriter, r *http.Request, request createListingRequest) (store.ListingInput, bool) {
	details := make([]FieldError, 0)
	title := strings.TrimSpace(request.Title)
	description := strings.TrimSpace(request.Description)
	if len([]rune(title)) < 10 || len([]rune(title)) > 120 {
		details = append(details, FieldError{Field: "title", Message: "title must have between 10 and 120 characters"})
	}
	if len([]rune(description)) < 20 || len([]rune(description)) > 5000 {
		details = append(details, FieldError{Field: "description", Message: "description must have between 20 and 5000 characters"})
	}
	categoryID, err := uuid.Parse(strings.TrimSpace(request.CategoryID))
	if err != nil {
		details = append(details, FieldError{Field: "category_id", Message: "category_id must be a UUID"})
	}
	if request.PriceCents == nil || *request.PriceCents <= 0 {
		details = append(details, FieldError{Field: "price_cents", Message: "price_cents must be a positive integer"})
	}
	if request.Currency != "BRL" {
		details = append(details, FieldError{Field: "currency", Message: "currency must be BRL"})
	}
	if request.Condition != "new" && request.Condition != "used" {
		details = append(details, FieldError{Field: "condition", Message: "condition must be new or used"})
	}
	if request.FreeShipping == nil {
		details = append(details, FieldError{Field: "free_shipping", Message: "free_shipping is required"})
	}
	if request.Stock == nil || *request.Stock < 0 || *request.Stock > 1000000 {
		details = append(details, FieldError{Field: "stock", Message: "stock must be between 0 and 1000000"})
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return store.ListingInput{}, false
	}
	return store.ListingInput{
		Title:        title,
		Description:  description,
		CategoryID:   categoryID,
		PriceCents:   *request.PriceCents,
		Currency:     request.Currency,
		Condition:    request.Condition,
		FreeShipping: *request.FreeShipping,
		Stock:        *request.Stock,
	}, true
}

func validateListingPatch(w http.ResponseWriter, r *http.Request, request updateListingRequest) (store.ListingPatch, bool) {
	details := make([]FieldError, 0)
	patch := store.ListingPatch{
		PriceCents:   request.PriceCents,
		FreeShipping: request.FreeShipping,
		Stock:        request.Stock,
	}
	if request.Title != nil {
		title := strings.TrimSpace(*request.Title)
		if len([]rune(title)) < 10 || len([]rune(title)) > 120 {
			details = append(details, FieldError{Field: "title", Message: "title must have between 10 and 120 characters"})
		}
		patch.Title = &title
	}
	if request.Description != nil {
		description := strings.TrimSpace(*request.Description)
		if len([]rune(description)) < 20 || len([]rune(description)) > 5000 {
			details = append(details, FieldError{Field: "description", Message: "description must have between 20 and 5000 characters"})
		}
		patch.Description = &description
	}
	if request.CategoryID != nil {
		categoryID, err := uuid.Parse(strings.TrimSpace(*request.CategoryID))
		if err != nil {
			details = append(details, FieldError{Field: "category_id", Message: "category_id must be a UUID"})
		} else {
			patch.CategoryID = &categoryID
		}
	}
	if request.PriceCents != nil && *request.PriceCents <= 0 {
		details = append(details, FieldError{Field: "price_cents", Message: "price_cents must be a positive integer"})
	}
	if request.Condition != nil && *request.Condition != "new" && *request.Condition != "used" {
		details = append(details, FieldError{Field: "condition", Message: "condition must be new or used"})
	} else if request.Condition != nil {
		patch.Condition = request.Condition
	}
	if request.Stock != nil && (*request.Stock < 0 || *request.Stock > 1000000) {
		details = append(details, FieldError{Field: "stock", Message: "stock must be between 0 and 1000000"})
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return store.ListingPatch{}, false
	}
	return patch, true
}

func (s *Server) currentSeller(w http.ResponseWriter, r *http.Request) (store.SellerProfile, bool) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return store.SellerProfile{}, false
	}
	profile, err := s.store.SellerByUserID(r.Context(), user.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusForbidden, "seller_profile_required", "seller onboarding is required")
		return store.SellerProfile{}, false
	}
	if err != nil {
		s.internalError(w, r, "select seller profile", err)
		return store.SellerProfile{}, false
	}
	return profile, true
}

func (s *Server) listSellerListings(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listings, err := s.store.SellerListings(r.Context(), profile.ID)
	if err != nil {
		s.internalError(w, r, "list seller listings", err)
		return
	}
	response := make([]sellerListingResponse, 0, len(listings))
	for _, listing := range listings {
		response = append(response, toSellerListing(listing))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) createListing(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	var request createListingRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	input, ok := validateListingCreate(w, r, request)
	if !ok {
		return
	}
	user, _ := CurrentUser(r)
	listing, err := s.store.CreateListing(r.Context(), profile.ID, user.ID, input)
	switch {
	case errors.Is(err, store.ErrCategoryInactive):
		writeValidationError(w, r, []FieldError{{Field: "category_id", Message: "category is inactive or does not exist"}})
	case err != nil:
		s.internalError(w, r, "create listing", err)
	default:
		s.refreshListingMetrics(r.Context())
		writeJSON(w, http.StatusCreated, toSellerListing(listing))
	}
}

func (s *Server) getSellerListing(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	listing, err := s.store.ListingByID(r.Context(), listingID)
	if errors.Is(err, store.ErrListingNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		s.internalError(w, r, "select listing", err)
		return
	}
	if listing.SellerID != profile.ID {
		writeError(w, r, http.StatusForbidden, "forbidden", "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, toSellerListing(listing))
}

func (s *Server) updateListing(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	var request updateListingRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	patch, ok := validateListingPatch(w, r, request)
	if !ok {
		return
	}
	user, _ := CurrentUser(r)
	listing, err := s.store.UpdateListing(r.Context(), listingID, profile.ID, user.ID, patch)
	s.writeListingResult(w, r, listing, err, http.StatusOK)
}

func (s *Server) submitListing(w http.ResponseWriter, r *http.Request) {
	s.sellerTransition(w, r, "submit", func(listingID, sellerID, actorID uuid.UUID) (store.Listing, error) {
		return s.store.SubmitListing(r.Context(), listingID, sellerID, actorID)
	})
}

func (s *Server) archiveListing(w http.ResponseWriter, r *http.Request) {
	s.sellerTransition(w, r, "archive", func(listingID, sellerID, actorID uuid.UUID) (store.Listing, error) {
		return s.store.ArchiveListing(r.Context(), listingID, sellerID, actorID)
	})
}

func (s *Server) sellerTransition(w http.ResponseWriter, r *http.Request, action string, transition func(uuid.UUID, uuid.UUID, uuid.UUID) (store.Listing, error)) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	user, _ := CurrentUser(r)
	listing, err := transition(listingID, profile.ID, user.ID)
	if err == nil {
		s.metrics.listingModerationDecisions.WithLabelValues(action).Inc()
		s.refreshListingMetrics(r.Context())
	}
	s.writeListingResult(w, r, listing, err, http.StatusOK)
}

func (s *Server) listPendingListings(w http.ResponseWriter, r *http.Request) {
	listings, err := s.store.PendingListings(r.Context())
	if err != nil {
		s.internalError(w, r, "list pending listings", err)
		return
	}
	response := make([]sellerListingResponse, 0, len(listings))
	for _, listing := range listings {
		response = append(response, toSellerListing(listing))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) publishListing(w http.ResponseWriter, r *http.Request) {
	s.adminTransition(w, r, "publish", func(listingID, actorID uuid.UUID) (store.Listing, error) {
		return s.store.PublishListing(r.Context(), listingID, actorID)
	})
}

func (s *Server) rejectListing(w http.ResponseWriter, r *http.Request) {
	s.adminModeration(w, r, "reject", func(listingID, actorID uuid.UUID, reason string) (store.Listing, error) {
		return s.store.RejectListing(r.Context(), listingID, actorID, reason)
	})
}

func (s *Server) suspendListing(w http.ResponseWriter, r *http.Request) {
	s.adminModeration(w, r, "suspend", func(listingID, actorID uuid.UUID, reason string) (store.Listing, error) {
		return s.store.SuspendListing(r.Context(), listingID, actorID, reason)
	})
}

func (s *Server) adminTransition(w http.ResponseWriter, r *http.Request, action string, transition func(uuid.UUID, uuid.UUID) (store.Listing, error)) {
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	user, _ := CurrentUser(r)
	listing, err := transition(listingID, user.ID)
	if err == nil {
		s.metrics.listingModerationDecisions.WithLabelValues(action).Inc()
		s.refreshListingMetrics(r.Context())
	}
	s.writeListingResult(w, r, listing, err, http.StatusOK)
}

func (s *Server) adminModeration(w http.ResponseWriter, r *http.Request, action string, transition func(uuid.UUID, uuid.UUID, string) (store.Listing, error)) {
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	var request moderationRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	reason := strings.TrimSpace(request.Reason)
	if len([]rune(reason)) < 3 || len([]rune(reason)) > 500 {
		writeValidationError(w, r, []FieldError{{Field: "reason", Message: "reason must have between 3 and 500 characters"}})
		return
	}
	user, _ := CurrentUser(r)
	listing, err := transition(listingID, user.ID, reason)
	if err == nil {
		s.metrics.listingModerationDecisions.WithLabelValues(action).Inc()
		s.refreshListingMetrics(r.Context())
	}
	s.writeListingResult(w, r, listing, err, http.StatusOK)
}

func (s *Server) writeListingResult(w http.ResponseWriter, r *http.Request, listing store.Listing, err error, status int) {
	switch {
	case errors.Is(err, store.ErrListingNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "listing not found")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, store.ErrInvalidState):
		writeError(w, r, http.StatusConflict, "invalid_listing_state", "listing state does not allow this operation")
	case errors.Is(err, store.ErrRequiresReview):
		writeError(w, r, http.StatusConflict, "listing_requires_review", "content changes require a new review")
	case errors.Is(err, store.ErrCategoryInactive):
		writeValidationError(w, r, []FieldError{{Field: "category_id", Message: "category is inactive or does not exist"}})
	case errors.Is(err, store.ErrInsufficientStock):
		writeError(w, r, http.StatusConflict, "insufficient_stock", "stock cannot become negative")
	case errors.Is(err, store.ErrInvalidListingData):
		writeError(w, r, http.StatusBadRequest, "invalid_listing_data", "invalid listing data")
	case err != nil:
		s.internalError(w, r, "listing operation", err)
	default:
		writeJSON(w, status, toSellerListing(listing))
	}
}

func listingIDFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "listingId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "listing not found")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	s.logger.Error(operation, "correlation_id", CorrelationID(r), "error", err)
	writeError(w, r, http.StatusInternalServerError, "internal_error", "internal error")
}

func (s *Server) refreshListingMetrics(ctx context.Context) {
	counts, err := s.store.ListingStatusCounts(ctx)
	if err != nil {
		s.logger.Warn("refresh listing metrics", "error", err)
		return
	}
	for _, status := range []string{"draft", "pending_review", "published", "rejected", "suspended", "archived"} {
		s.metrics.listingsByStatus.WithLabelValues(status).Set(float64(counts[status]))
	}
}
