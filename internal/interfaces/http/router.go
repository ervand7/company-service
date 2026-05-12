package http

import (
	"context"
	nethttp "net/http"
	"time"

	appcompany "company-service/internal/application/company"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	httpSwagger "github.com/swaggo/http-swagger"
)

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

func NewRouter(service *appcompany.Service, authValidator TokenValidator, ready ReadinessChecker, logger zerolog.Logger) nethttp.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	handler := NewHandler(service, logger)

	r.Get("/swagger/*", httpSwagger.WrapHandler)
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(ready))

	r.Get("/companies/{id}", handler.GetCompany)

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(authValidator))
		r.Post("/companies", handler.CreateCompany)
		r.Patch("/companies/{id}", handler.PatchCompany)
		r.Delete("/companies/{id}", handler.DeleteCompany)
	})

	return r
}

// healthz godoc
// @Summary Health check
// @Description Returns a simple status response when the process is running.
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /healthz [get]
func healthz(w nethttp.ResponseWriter, _ *nethttp.Request) {
	writeJSON(w, nethttp.StatusOK, map[string]string{"status": "ok"})
}

// readyz godoc
// @Summary Readiness check
// @Description Checks whether the service can reach its database dependency.
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} errorResponse
// @Router /readyz [get]
func readyz(ready ReadinessChecker) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := ready.Ping(ctx); err != nil {
			writeJSON(w, nethttp.StatusServiceUnavailable, errorResponse{Error: "not ready"})
			return
		}
		writeJSON(w, nethttp.StatusOK, map[string]string{"status": "ok"})
	}
}
