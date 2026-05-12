package http

import (
	"net/http"
	"strings"
)

type TokenValidator interface {
	Validate(token string) (any, error)
}

func AuthMiddleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "missing bearer token"})
				return
			}

			if _, err := validator.Validate(token); err != nil {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid bearer token"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
