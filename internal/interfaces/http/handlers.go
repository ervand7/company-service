package http

import (
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"

	appcompany "company-service/internal/application/company"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Handler struct {
	service *appcompany.Service
	logger  zerolog.Logger
}

func NewHandler(service *appcompany.Service, logger zerolog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// CreateCompany godoc
// @Summary Create a company
// @Description Creates a company and publishes a company.created event.
// @Tags companies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param company body createCompanyRequest true "Company payload"
// @Success 201 {object} companyResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /companies [post]
func (h *Handler) CreateCompany(w nethttp.ResponseWriter, r *nethttp.Request) {
	var req createCompanyRequest
	if err := decodeJSON(r, &req); err != nil {
		h.logError(r, nethttp.StatusBadRequest, err, "failed to decode create company request")
		writeJSON(w, nethttp.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	input, err := req.toInput()
	if err != nil {
		h.logError(r, nethttp.StatusBadRequest, err, "invalid create company request")
		writeAppError(w, err)
		return
	}

	company, err := h.service.CreateCompany(r.Context(), input)
	if err != nil {
		h.logError(r, appErrorStatus(err), err, "failed to create company")
		writeAppError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusCreated, toCompanyResponse(company))
}

// PatchCompany godoc
// @Summary Patch a company
// @Description Partially updates an existing company and publishes a company.updated event.
// @Tags companies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Company ID" Format(uuid)
// @Param company body patchCompanyRequest true "Fields to update"
// @Success 200 {object} companyResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /companies/{id} [patch]
func (h *Handler) PatchCompany(w nethttp.ResponseWriter, r *nethttp.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	var req patchCompanyRequest
	if err := decodeJSON(r, &req); err != nil {
		h.logError(r, nethttp.StatusBadRequest, err, "failed to decode patch company request")
		writeJSON(w, nethttp.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	input, err := req.toInput()
	if err != nil {
		h.logError(r, nethttp.StatusBadRequest, err, "invalid patch company request")
		writeAppError(w, err)
		return
	}

	company, err := h.service.PatchCompany(r.Context(), id, input)
	if err != nil {
		h.logError(r, appErrorStatus(err), err, "failed to patch company")
		writeAppError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, toCompanyResponse(company))
}

// DeleteCompany godoc
// @Summary Delete a company
// @Description Deletes an existing company and publishes a company.deleted event.
// @Tags companies
// @Produce json
// @Security BearerAuth
// @Param id path string true "Company ID" Format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /companies/{id} [delete]
func (h *Handler) DeleteCompany(w nethttp.ResponseWriter, r *nethttp.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	if err := h.service.DeleteCompany(r.Context(), id); err != nil {
		h.logError(r, appErrorStatus(err), err, "failed to delete company")
		writeAppError(w, err)
		return
	}

	w.WriteHeader(nethttp.StatusNoContent)
}

// GetCompany godoc
// @Summary Get a company
// @Description Returns a company by ID. This endpoint is public.
// @Tags companies
// @Produce json
// @Param id path string true "Company ID" Format(uuid)
// @Success 200 {object} companyResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /companies/{id} [get]
func (h *Handler) GetCompany(w nethttp.ResponseWriter, r *nethttp.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	company, err := h.service.GetCompany(r.Context(), id)
	if err != nil {
		h.logError(r, appErrorStatus(err), err, "failed to get company")
		writeAppError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, toCompanyResponse(company))
}

func (h *Handler) parseID(w nethttp.ResponseWriter, r *nethttp.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.logError(r, nethttp.StatusBadRequest, err, "invalid company id")
		writeJSON(w, nethttp.StatusBadRequest, errorResponse{Error: "invalid company id"})
		return uuid.Nil, false
	}

	return id, true
}

func (h *Handler) logError(r *nethttp.Request, status int, err error, message string) {
	event := h.logger.Error().
		Err(err).
		Int("status", status).
		Str("method", r.Method).
		Str("path", r.URL.Path)
	if requestID := middleware.GetReqID(r.Context()); requestID != "" {
		event = event.Str("request_id", requestID)
	}
	event.Msg(message)
}

func decodeJSON(r *nethttp.Request, target any) error {
	if r.Body == nil {
		return errors.New("missing body")
	}
	defer func() {
		_ = r.Body.Close()
	}()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single JSON object")
	}

	return nil
}
