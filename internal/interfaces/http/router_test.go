package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcompany "company-service/internal/application/company"
	"company-service/internal/application/company/mocks"
	companydomain "company-service/internal/domain/company"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

type routerTestValidator struct {
	err error
}

func (v routerTestValidator) Validate(token string) (any, error) {
	if token != "valid-token" {
		return nil, errors.New("invalid token")
	}
	return "subject", v.err
}

type routerTestReadiness struct {
	err         error
	called      bool
	hasDeadline bool
}

func (r *routerTestReadiness) Ping(ctx context.Context) error {
	r.called = true
	_, r.hasDeadline = ctx.Deadline()
	return r.err
}

func TestRouterHealthzReturnsOK(t *testing.T) {
	router := NewRouter(nil, routerTestValidator{}, &routerTestReadiness{}, zerolog.Nop())

	rec := serveRouter(router, nethttp.MethodGet, "/healthz", "", "")

	assertJSONMap(t, rec, nethttp.StatusOK, "status", "ok")
}

func TestRouterReadyzChecksReadiness(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ready",
			wantStatus: nethttp.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "not ready",
			err:        errors.New("database unavailable"),
			wantStatus: nethttp.StatusServiceUnavailable,
			wantBody:   "not ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := &routerTestReadiness{err: tt.err}
			router := NewRouter(nil, routerTestValidator{}, ready, zerolog.Nop())

			rec := serveRouter(router, nethttp.MethodGet, "/readyz", "", "")

			if !ready.called {
				t.Fatal("expected readiness checker to be called")
			}
			if !ready.hasDeadline {
				t.Fatal("expected readiness checker context to have a deadline")
			}
			if tt.err != nil {
				assertJSONError(t, rec, tt.wantStatus, tt.wantBody)
				return
			}
			assertJSONMap(t, rec, tt.wantStatus, "status", tt.wantBody)
		})
	}
}

func TestRouterGetCompanyIsPublic(t *testing.T) {
	service, repo, events := newRouterTestService(t)
	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(testCompany(t, id, "Acme"), nil).Once()
	router := NewRouter(service, routerTestValidator{}, &routerTestReadiness{}, zerolog.Nop())

	rec := serveRouter(router, nethttp.MethodGet, "/companies/"+id.String(), "", "")

	assertJSONCompany(t, rec, nethttp.StatusOK, "Acme")
	events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestRouterProtectedCompanyRoutesRequireBearerToken(t *testing.T) {
	router := NewRouter(nil, routerTestValidator{}, &routerTestReadiness{}, zerolog.Nop())

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create company",
			method: nethttp.MethodPost,
			path:   "/companies",
			body:   validCreateCompanyJSON("Acme"),
		},
		{
			name:   "patch company",
			method: nethttp.MethodPatch,
			path:   "/companies/" + uuid.NewString(),
			body:   `{"description":"updated"}`,
		},
		{
			name:   "delete company",
			method: nethttp.MethodDelete,
			path:   "/companies/" + uuid.NewString(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveRouter(router, tt.method, tt.path, tt.body, "")

			assertJSONError(t, rec, nethttp.StatusUnauthorized, "missing bearer token")
		})
	}
}

func TestRouterCreateCompanyAcceptsValidBearerToken(t *testing.T) {
	service, repo, events := newRouterTestService(t)
	repo.On("GetByName", mock.Anything, "Acme").Return(nil, companydomain.ErrNotFound).Once()
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *companydomain.Company) bool {
		return c.Name == "Acme"
	})).Return(nil).Once()
	events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyCreated && event.Company.Name == "Acme"
	})).Return(nil).Once()
	router := NewRouter(service, routerTestValidator{}, &routerTestReadiness{}, zerolog.Nop())

	rec := serveRouter(router, nethttp.MethodPost, "/companies", validCreateCompanyJSON("Acme"), "Bearer valid-token")

	assertJSONCompany(t, rec, nethttp.StatusCreated, "Acme")
}

func newRouterTestService(t *testing.T) (*appcompany.Service, *mocks.Repository, *mocks.EventProducer) {
	t.Helper()

	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	logger := zerolog.Nop()

	return appcompany.NewService(repo, events, &logger), repo, events
}

func serveRouter(handler nethttp.Handler, method, path, body, authorization string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	return rec
}

func assertJSONMap(t *testing.T, rec *httptest.ResponseRecorder, status int, key, value string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("expected status %d, got %d with body %q", status, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body[key] != value {
		t.Fatalf("expected %s %q, got %q", key, value, body[key])
	}
}
