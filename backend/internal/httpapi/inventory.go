package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"mercado/internal/store"
)

func (s *Server) listInventoryAdjustments(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	adjustments, err := s.store.InventoryAdjustments(r.Context(), listingID, profile.ID)
	if err != nil {
		s.writeInventoryError(w, r, err)
		return
	}
	response := make([]inventoryAdjustmentResponse, 0, len(adjustments))
	for _, adjustment := range adjustments {
		response = append(response, toInventoryAdjustment(adjustment))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) adjustInventory(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.currentSeller(w, r)
	if !ok {
		return
	}
	listingID, ok := listingIDFromRequest(w, r)
	if !ok {
		return
	}
	var request inventoryAdjustmentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	reason := strings.TrimSpace(request.Reason)
	details := make([]FieldError, 0)
	if request.Delta == nil || *request.Delta == 0 {
		details = append(details, FieldError{Field: "delta", Message: "delta must be a non-zero integer"})
	}
	if len([]rune(reason)) < 3 || len([]rune(reason)) > 200 {
		details = append(details, FieldError{Field: "reason", Message: "reason must have between 3 and 200 characters"})
	}
	if len(details) > 0 {
		writeValidationError(w, r, details)
		return
	}

	user, _ := CurrentUser(r)
	adjustment, err := s.store.AdjustInventory(r.Context(), listingID, profile.ID, user.ID, *request.Delta, reason)
	if err != nil {
		s.writeInventoryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toInventoryAdjustment(adjustment))
}

func (s *Server) writeInventoryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrListingNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "listing not found")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, store.ErrInvalidState):
		writeError(w, r, http.StatusConflict, "invalid_listing_state", "listing state does not allow this operation")
	case errors.Is(err, store.ErrInsufficientStock):
		writeError(w, r, http.StatusConflict, "insufficient_stock", "stock cannot become negative")
	case errors.Is(err, store.ErrInvalidListingData):
		writeError(w, r, http.StatusBadRequest, "invalid_inventory_adjustment", "invalid inventory adjustment")
	case err != nil:
		s.internalError(w, r, "inventory operation", err)
	}
}
