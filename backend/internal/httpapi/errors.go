package httpapi

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error         ErrorDetail `json:"error"`
	CorrelationID string      `json:"correlation_id,omitempty"`
}

type ErrorDetail struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
		CorrelationID: CorrelationID(r),
	})
}

func writeValidationError(w http.ResponseWriter, r *http.Request, details []FieldError) {
	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Error: ErrorDetail{
			Code:    "validation_error",
			Message: "request validation failed",
			Details: details,
		},
		CorrelationID: CorrelationID(r),
	})
}

func writeErrorWithDetails(w http.ResponseWriter, r *http.Request, status int, code, message string, details []FieldError) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
		CorrelationID: CorrelationID(r),
	})
}
