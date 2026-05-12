package http

import (
	"encoding/json"
	"errors"
	nethttp "net/http"

	companydomain "company-service/internal/domain/company"
)

type errorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

func writeJSON(w nethttp.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if value == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}

func writeAppError(w nethttp.ResponseWriter, err error) {
	var validationErr companydomain.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeJSON(w, nethttp.StatusBadRequest, errorResponse{Error: "validation failed", Fields: validationErr.Fields})
	case errors.Is(err, companydomain.ErrEmptyPatch):
		writeJSON(w, nethttp.StatusBadRequest, errorResponse{Error: "empty patch body"})
	case errors.Is(err, companydomain.ErrNotFound):
		writeJSON(w, nethttp.StatusNotFound, errorResponse{Error: "company not found"})
	case errors.Is(err, companydomain.ErrDuplicateName):
		writeJSON(w, nethttp.StatusConflict, errorResponse{Error: "company name already exists"})
	default:
		writeJSON(w, nethttp.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}

func appErrorStatus(err error) int {
	var validationErr companydomain.ValidationError
	switch {
	case errors.As(err, &validationErr), errors.Is(err, companydomain.ErrEmptyPatch):
		return nethttp.StatusBadRequest
	case errors.Is(err, companydomain.ErrNotFound):
		return nethttp.StatusNotFound
	case errors.Is(err, companydomain.ErrDuplicateName):
		return nethttp.StatusConflict
	default:
		return nethttp.StatusInternalServerError
	}
}
