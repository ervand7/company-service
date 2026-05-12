package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	appcompany "company-service/internal/application/company"
	"company-service/internal/application/company/mocks"
	companydomain "company-service/internal/domain/company"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

type handlerTestApp struct {
	handler *Handler
	repo    *mocks.Repository
	events  *mocks.EventProducer
	logs    *bytes.Buffer
}

func TestCreateCompanyHandlerSuccess(t *testing.T) {
	app := newHandlerTestApp(t)

	app.repo.On("GetByName", mock.Anything, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	app.repo.On("Create", mock.Anything, mock.MatchedBy(func(c *companydomain.Company) bool {
		return c.Name == "Acme" &&
			c.Description == "description" &&
			c.AmountOfEmployees == 10 &&
			c.Registered &&
			c.Type == companydomain.TypeCorporations
	})).Return(nil).Once()
	app.events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyCreated &&
			event.Company != nil &&
			event.CompanyID == event.Company.ID
	})).Return(nil).Once()

	rec := app.serve(nethttp.MethodPost, "/companies", `{
		"id": "00000000-0000-0000-0000-000000000001",
		"name": "Acme",
		"description": "description",
		"amount_of_employees": 10,
		"registered": true,
		"type": "Corporations"
	}`)

	assertJSONCompany(t, rec, nethttp.StatusCreated, "Acme")
	assertNoLogs(t, app.logs)
}

func TestCreateCompanyHandlerRejectsInvalidJSON(t *testing.T) {
	app := newHandlerTestApp(t)

	rec := app.serve(nethttp.MethodPost, "/companies", `{`)

	assertJSONError(t, rec, nethttp.StatusBadRequest, "invalid JSON body")
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "failed to decode create company request")
}

func TestCreateCompanyHandlerRejectsValidationError(t *testing.T) {
	app := newHandlerTestApp(t)

	rec := app.serve(nethttp.MethodPost, "/companies", `{}`)

	body := assertJSONError(t, rec, nethttp.StatusBadRequest, "validation failed")
	for _, field := range []string{"id", "name", "amount_of_employees", "registered", "type"} {
		if body.Fields[field] != "is required" {
			t.Fatalf("expected required validation for %s, got %q", field, body.Fields[field])
		}
	}
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "invalid create company request")
}

func TestCreateCompanyHandlerReturnsDuplicateName(t *testing.T) {
	app := newHandlerTestApp(t)
	existing := testCompany(t, uuid.New(), "Acme")

	app.repo.On("GetByName", mock.Anything, "Acme").Return(existing, nil).Once()

	rec := app.serve(nethttp.MethodPost, "/companies", validCreateCompanyJSON("Acme"))

	assertJSONError(t, rec, nethttp.StatusConflict, "company name already exists")
	assertSingleLog(t, app.logs, nethttp.StatusConflict, "failed to create company")
	app.repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyHandlerReturnsDuplicateID(t *testing.T) {
	app := newHandlerTestApp(t)

	app.repo.On("GetByName", mock.Anything, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	app.repo.On("Create", mock.Anything, mock.AnythingOfType("*company.Company")).Return(companydomain.ErrDuplicateID).Once()

	rec := app.serve(nethttp.MethodPost, "/companies", validCreateCompanyJSON("Acme"))

	assertJSONError(t, rec, nethttp.StatusConflict, "company id already exists")
	assertSingleLog(t, app.logs, nethttp.StatusConflict, "failed to create company")
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyHandlerReturnsLookupError(t *testing.T) {
	app := newHandlerTestApp(t)

	app.repo.On("GetByName", mock.Anything, "Acme").Return((*companydomain.Company)(nil), errors.New("lookup failed")).Once()

	rec := app.serve(nethttp.MethodPost, "/companies", validCreateCompanyJSON("Acme"))

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to create company")
	app.repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyHandlerReturnsCreateError(t *testing.T) {
	app := newHandlerTestApp(t)

	app.repo.On("GetByName", mock.Anything, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	app.repo.On("Create", mock.Anything, mock.AnythingOfType("*company.Company")).Return(errors.New("create failed")).Once()

	rec := app.serve(nethttp.MethodPost, "/companies", validCreateCompanyJSON("Acme"))

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to create company")
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestCreateCompanyHandlerReturnsPublishError(t *testing.T) {
	app := newHandlerTestApp(t)

	app.repo.On("GetByName", mock.Anything, "Acme").Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()
	app.repo.On("Create", mock.Anything, mock.AnythingOfType("*company.Company")).Return(nil).Once()
	app.events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyCreated
	})).Return(errors.New("publish failed")).Once()

	rec := app.serve(nethttp.MethodPost, "/companies", validCreateCompanyJSON("Acme"))

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to create company")
}

func TestPatchCompanyHandlerSuccess(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("Update", mock.Anything, mock.MatchedBy(func(c *companydomain.Company) bool {
		return c.ID == id &&
			c.Name == "Acme" &&
			c.Description == "updated" &&
			c.AmountOfEmployees == 20 &&
			!c.Registered &&
			c.Type == companydomain.TypeNonProfit
	})).Return(nil).Once()
	app.events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyUpdated && event.CompanyID == id
	})).Return(nil).Once()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{
		"description": "updated",
		"amount_of_employees": 20,
		"registered": false,
		"type": "NonProfit"
	}`)

	assertJSONCompany(t, rec, nethttp.StatusOK, "Acme")
	assertNoLogs(t, app.logs)
}

func TestPatchCompanyHandlerRejectsInvalidID(t *testing.T) {
	app := newHandlerTestApp(t)

	rec := app.serve(nethttp.MethodPatch, "/companies/not-a-uuid", `{}`)

	assertJSONError(t, rec, nethttp.StatusBadRequest, "invalid company id")
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "invalid company id")
}

func TestPatchCompanyHandlerRejectsInvalidJSON(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{`)

	assertJSONError(t, rec, nethttp.StatusBadRequest, "invalid JSON body")
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "failed to decode patch company request")
}

func TestPatchCompanyHandlerRejectsEmptyPatch(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{}`)

	assertJSONError(t, rec, nethttp.StatusBadRequest, "empty patch body")
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "failed to patch company")
	app.repo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestPatchCompanyHandlerReturnsGetByIDError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()

	app.repo.On("GetByID", mock.Anything, id).Return((*companydomain.Company)(nil), errors.New("get failed")).Once()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{"description":"updated"}`)

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to patch company")
	app.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyHandlerReturnsDuplicateName(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")
	other := testCompany(t, uuid.New(), "Other")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("GetByName", mock.Anything, "Other").Return(other, nil).Once()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{"name":"Other"}`)

	assertJSONError(t, rec, nethttp.StatusConflict, "company name already exists")
	assertSingleLog(t, app.logs, nethttp.StatusConflict, "failed to patch company")
	app.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyHandlerReturnsNameLookupError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("GetByName", mock.Anything, "Other").Return((*companydomain.Company)(nil), errors.New("lookup failed")).Once()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{"name":"Other"}`)

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to patch company")
	app.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyHandlerReturnsValidationError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()

	body := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{"amount_of_employees":-1}`)

	errBody := assertJSONError(t, body, nethttp.StatusBadRequest, "validation failed")
	if errBody.Fields["amount_of_employees"] == "" {
		t.Fatal("expected amount_of_employees validation error")
	}
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "failed to patch company")
	app.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyHandlerReturnsUpdateError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("Update", mock.Anything, mock.AnythingOfType("*company.Company")).Return(errors.New("update failed")).Once()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{"description":"updated"}`)

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to patch company")
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestPatchCompanyHandlerReturnsPublishError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("Update", mock.Anything, mock.AnythingOfType("*company.Company")).Return(nil).Once()
	app.events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyUpdated && event.CompanyID == id
	})).Return(errors.New("publish failed")).Once()

	rec := app.serve(nethttp.MethodPatch, "/companies/"+id.String(), `{"description":"updated"}`)

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to patch company")
}

func TestDeleteCompanyHandlerSuccess(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("Delete", mock.Anything, id).Return(nil).Once()
	app.events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyDeleted && event.CompanyID == id
	})).Return(nil).Once()

	rec := app.serve(nethttp.MethodDelete, "/companies/"+id.String(), "")

	if rec.Code != nethttp.StatusNoContent {
		t.Fatalf("expected status %d, got %d", nethttp.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", rec.Body.String())
	}
	assertNoLogs(t, app.logs)
}

func TestDeleteCompanyHandlerRejectsInvalidID(t *testing.T) {
	app := newHandlerTestApp(t)

	rec := app.serve(nethttp.MethodDelete, "/companies/not-a-uuid", "")

	assertJSONError(t, rec, nethttp.StatusBadRequest, "invalid company id")
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "invalid company id")
}

func TestDeleteCompanyHandlerReturnsNotFound(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()

	app.repo.On("GetByID", mock.Anything, id).Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()

	rec := app.serve(nethttp.MethodDelete, "/companies/"+id.String(), "")

	assertJSONError(t, rec, nethttp.StatusNotFound, "company not found")
	assertSingleLog(t, app.logs, nethttp.StatusNotFound, "failed to delete company")
	app.repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestDeleteCompanyHandlerReturnsDeleteError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("Delete", mock.Anything, id).Return(errors.New("delete failed")).Once()

	rec := app.serve(nethttp.MethodDelete, "/companies/"+id.String(), "")

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to delete company")
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestDeleteCompanyHandlerReturnsPublishError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()
	app.repo.On("Delete", mock.Anything, id).Return(nil).Once()
	app.events.On("Publish", mock.Anything, mock.MatchedBy(func(event appcompany.Event) bool {
		return event.Type == appcompany.EventCompanyDeleted && event.CompanyID == id
	})).Return(errors.New("publish failed")).Once()

	rec := app.serve(nethttp.MethodDelete, "/companies/"+id.String(), "")

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to delete company")
}

func TestGetCompanyHandlerSuccess(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()
	existing := testCompany(t, id, "Acme")

	app.repo.On("GetByID", mock.Anything, id).Return(existing, nil).Once()

	rec := app.serve(nethttp.MethodGet, "/companies/"+id.String(), "")

	assertJSONCompany(t, rec, nethttp.StatusOK, "Acme")
	assertNoLogs(t, app.logs)
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestGetCompanyHandlerRejectsInvalidID(t *testing.T) {
	app := newHandlerTestApp(t)

	rec := app.serve(nethttp.MethodGet, "/companies/not-a-uuid", "")

	assertJSONError(t, rec, nethttp.StatusBadRequest, "invalid company id")
	assertSingleLog(t, app.logs, nethttp.StatusBadRequest, "invalid company id")
}

func TestGetCompanyHandlerReturnsNotFound(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()

	app.repo.On("GetByID", mock.Anything, id).Return((*companydomain.Company)(nil), companydomain.ErrNotFound).Once()

	rec := app.serve(nethttp.MethodGet, "/companies/"+id.String(), "")

	assertJSONError(t, rec, nethttp.StatusNotFound, "company not found")
	assertSingleLog(t, app.logs, nethttp.StatusNotFound, "failed to get company")
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestGetCompanyHandlerReturnsRepositoryError(t *testing.T) {
	app := newHandlerTestApp(t)
	id := uuid.New()

	app.repo.On("GetByID", mock.Anything, id).Return((*companydomain.Company)(nil), errors.New("get failed")).Once()

	rec := app.serve(nethttp.MethodGet, "/companies/"+id.String(), "")

	assertJSONError(t, rec, nethttp.StatusInternalServerError, "internal server error")
	assertSingleLog(t, app.logs, nethttp.StatusInternalServerError, "failed to get company")
	app.events.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestDecodeJSONRejectsMalformedBodies(t *testing.T) {
	tests := []struct {
		name string
		req  *nethttp.Request
	}{
		{
			name: "missing body",
			req:  &nethttp.Request{},
		},
		{
			name: "unknown field",
			req:  httptest.NewRequest(nethttp.MethodPost, "/companies", strings.NewReader(`{"unknown":true}`)),
		},
		{
			name: "multiple JSON objects",
			req:  httptest.NewRequest(nethttp.MethodPost, "/companies", strings.NewReader(`{} {}`)),
		},
		{
			name: "empty body",
			req:  httptest.NewRequest(nethttp.MethodPost, "/companies", strings.NewReader(``)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req createCompanyRequest
			if err := decodeJSON(tt.req, &req); err == nil {
				t.Fatal("expected decode error")
			}
		})
	}
}

func TestDecodeJSONAcceptsSingleJSONObject(t *testing.T) {
	req := httptest.NewRequest(nethttp.MethodPost, "/companies", strings.NewReader(validCreateCompanyJSON("Acme")))

	var body createCompanyRequest
	if err := decodeJSON(req, &body); err != nil {
		t.Fatal(err)
	}
	if body.Name == nil || *body.Name != "Acme" {
		t.Fatalf("expected decoded company name, got %+v", body.Name)
	}
}

func newHandlerTestApp(t *testing.T) handlerTestApp {
	t.Helper()

	logs := &bytes.Buffer{}
	logger := zerolog.New(logs)
	repo := mocks.NewRepository(t)
	events := mocks.NewEventProducer(t)
	service := appcompany.NewService(repo, events, &logger)

	return handlerTestApp{
		handler: NewHandler(service, logger),
		repo:    repo,
		events:  events,
		logs:    logs,
	}
}

func (a handlerTestApp) serve(method, path, body string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Post("/companies", a.handler.CreateCompany)
	router.Patch("/companies/{id}", a.handler.PatchCompany)
	router.Delete("/companies/{id}", a.handler.DeleteCompany)
	router.Get("/companies/{id}", a.handler.GetCompany)

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	return rec
}

func validCreateCompanyJSON(name string) string {
	return `{"id":` + strconvQuote(uuid.NewString()) + `,"name":` + strconvQuote(name) + `,"description":"description","amount_of_employees":10,"registered":true,"type":"Corporations"}`
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func testCompany(t *testing.T, id uuid.UUID, name string) *companydomain.Company {
	t.Helper()

	company, err := companydomain.New(name, "description", 10, true, companydomain.TypeCorporations)
	if err != nil {
		t.Fatal(err)
	}
	company.ID = id

	return company
}

func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) errorResponse {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("expected status %d, got %d with body %q", status, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != message {
		t.Fatalf("expected error %q, got %q", message, body.Error)
	}

	return body
}

func assertJSONCompany(t *testing.T, rec *httptest.ResponseRecorder, status int, name string) companyResponse {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("expected status %d, got %d with body %q", status, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var body companyResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ID == "" {
		t.Fatal("expected response id")
	}
	if body.Name != name {
		t.Fatalf("expected company name %q, got %q", name, body.Name)
	}

	return body
}

func assertNoLogs(t *testing.T, logs *bytes.Buffer) {
	t.Helper()

	if logs.Len() != 0 {
		t.Fatalf("expected no logs, got %q", logs.String())
	}
}

func assertSingleLog(t *testing.T, logs *bytes.Buffer, status int, message string) {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("expected one log line, got %d: %q", len(lines), logs.String())
	}
	if !strings.Contains(lines[0], strconvQuote(message)) {
		t.Fatalf("expected log message %q, got %q", message, lines[0])
	}
	if !strings.Contains(lines[0], `"status":`+strconv.Itoa(status)) {
		t.Fatalf("expected log status %d, got %q", status, lines[0])
	}
}
