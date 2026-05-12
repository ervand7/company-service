package http

import (
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	companydomain "company-service/internal/domain/company"
)

func TestWriteJSONWritesStatusContentTypeAndBody(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, nethttp.StatusCreated, map[string]string{"status": "created"})

	if rec.Code != nethttp.StatusCreated {
		t.Fatalf("expected status %d, got %d", nethttp.StatusCreated, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "created" {
		t.Fatalf("expected status body %q, got %q", "created", body["status"])
	}
}

func TestWriteJSONWithNilBodyWritesOnlyHeaders(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, nethttp.StatusNoContent, nil)

	if rec.Code != nethttp.StatusNoContent {
		t.Fatalf("expected status %d, got %d", nethttp.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestWriteAppError(t *testing.T) {
	validationErr := companydomain.ValidationError{
		Fields: map[string]string{"name": "is required"},
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   errorResponse
	}{
		{
			name:       "validation error",
			err:        fmt.Errorf("create company: %w", validationErr),
			wantStatus: nethttp.StatusBadRequest,
			wantBody: errorResponse{
				Error:  "validation failed",
				Fields: validationErr.Fields,
			},
		},
		{
			name:       "empty patch",
			err:        fmt.Errorf("patch company: %w", companydomain.ErrEmptyPatch),
			wantStatus: nethttp.StatusBadRequest,
			wantBody:   errorResponse{Error: "empty patch body"},
		},
		{
			name:       "not found",
			err:        fmt.Errorf("get company: %w", companydomain.ErrNotFound),
			wantStatus: nethttp.StatusNotFound,
			wantBody:   errorResponse{Error: "company not found"},
		},
		{
			name:       "duplicate name",
			err:        fmt.Errorf("create company: %w", companydomain.ErrDuplicateName),
			wantStatus: nethttp.StatusConflict,
			wantBody:   errorResponse{Error: "company name already exists"},
		},
		{
			name:       "unknown error",
			err:        errors.New("database unavailable"),
			wantStatus: nethttp.StatusInternalServerError,
			wantBody:   errorResponse{Error: "internal server error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeAppError(rec, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected JSON content type, got %q", got)
			}

			var body errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Error != tt.wantBody.Error {
				t.Fatalf("expected error %q, got %q", tt.wantBody.Error, body.Error)
			}
			if len(tt.wantBody.Fields) == 0 && len(body.Fields) != 0 {
				t.Fatalf("expected no fields, got %+v", body.Fields)
			}
			for field, message := range tt.wantBody.Fields {
				if body.Fields[field] != message {
					t.Fatalf("expected field %s to be %q, got %q", field, message, body.Fields[field])
				}
			}
		})
	}
}

func TestAppErrorStatus(t *testing.T) {
	validationErr := companydomain.ValidationError{
		Fields: map[string]string{"type": "is required"},
	}

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "validation error",
			err:  fmt.Errorf("create company: %w", validationErr),
			want: nethttp.StatusBadRequest,
		},
		{
			name: "empty patch",
			err:  fmt.Errorf("patch company: %w", companydomain.ErrEmptyPatch),
			want: nethttp.StatusBadRequest,
		},
		{
			name: "not found",
			err:  fmt.Errorf("get company: %w", companydomain.ErrNotFound),
			want: nethttp.StatusNotFound,
		},
		{
			name: "duplicate name",
			err:  fmt.Errorf("create company: %w", companydomain.ErrDuplicateName),
			want: nethttp.StatusConflict,
		},
		{
			name: "unknown error",
			err:  errors.New("unexpected"),
			want: nethttp.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appErrorStatus(tt.err); got != tt.want {
				t.Fatalf("expected status %d, got %d", tt.want, got)
			}
		})
	}
}
