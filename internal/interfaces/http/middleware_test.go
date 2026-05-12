package http_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	httpapi "company-service/internal/interfaces/http"
)

type fakeValidator struct {
	err error
}

func (v fakeValidator) Validate(token string) (any, error) {
	if token != "valid-token" {
		return nil, errors.New("invalid token")
	}
	return "subject", v.err
}

func TestAuthMiddlewareAllowsValidBearerToken(t *testing.T) {
	handler := httpapi.AuthMiddleware(fakeValidator{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/companies", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	handler := httpapi.AuthMiddleware(fakeValidator{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/companies", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
