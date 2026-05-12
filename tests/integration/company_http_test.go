//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	appcompany "company-service/internal/application/company"
	"company-service/internal/config"
	"company-service/internal/infrastructure/auth"
	"company-service/internal/infrastructure/postgres"
	httpapi "company-service/internal/interfaces/http"

	"github.com/rs/zerolog"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testJWTSecret = "integration-test-secret"

type noopProducer struct{}

func (noopProducer) Publish(context.Context, appcompany.Event) error {
	return nil
}

func (noopProducer) Close(context.Context) error {
	return nil
}

type companyResponse struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AmountOfEmployees int    `json:"amount_of_employees"`
	Registered        bool   `json:"registered"`
	Type              string `json:"type"`
}

type testApp struct {
	server *httptest.Server
	token  string
}

func TestCompanyHTTPIntegration(t *testing.T) {
	app := setupTestApp(t)

	t.Run("protected endpoints without token return 401", func(t *testing.T) {
		status, _ := app.do(t, http.MethodPost, "/companies", nil, "")
		if status != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", status)
		}
	})

	t.Run("create validation errors return 400", func(t *testing.T) {
		status, _ := app.do(t, http.MethodPost, "/companies", map[string]any{
			"name":                "",
			"amount_of_employees": -1,
			"registered":          true,
			"type":                "Invalid",
		}, app.token)
		if status != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", status)
		}
	})

	var created companyResponse
	t.Run("create success", func(t *testing.T) {
		status, body := app.do(t, http.MethodPost, "/companies", map[string]any{
			"name":                "Acme",
			"description":         "Original",
			"amount_of_employees": 10,
			"registered":          true,
			"type":                "Corporations",
		}, app.token)
		if status != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", status, body)
		}
		mustDecode(t, body, &created)
		if created.ID == "" || created.Name != "Acme" {
			t.Fatalf("unexpected create response: %+v", created)
		}
	})

	t.Run("duplicate name returns 409", func(t *testing.T) {
		status, _ := app.do(t, http.MethodPost, "/companies", map[string]any{
			"name":                "Acme",
			"amount_of_employees": 5,
			"registered":          true,
			"type":                "NonProfit",
		}, app.token)
		if status != http.StatusConflict {
			t.Fatalf("expected 409, got %d", status)
		}
	})

	t.Run("get not found returns 404", func(t *testing.T) {
		status, _ := app.do(t, http.MethodGet, "/companies/00000000-0000-0000-0000-000000000001", nil, "")
		if status != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", status)
		}
	})

	t.Run("patch partial update does not overwrite missing fields", func(t *testing.T) {
		status, body := app.do(t, http.MethodPatch, "/companies/"+created.ID, map[string]any{
			"registered": false,
		}, app.token)
		if status != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", status, body)
		}

		var patched companyResponse
		mustDecode(t, body, &patched)
		if patched.Name != "Acme" || patched.Description != "Original" || patched.AmountOfEmployees != 10 {
			t.Fatalf("missing fields were overwritten: %+v", patched)
		}
		if patched.Registered {
			t.Fatal("expected registered to be false")
		}
	})

	t.Run("delete returns 204 and deleted company returns 404", func(t *testing.T) {
		status, body := app.do(t, http.MethodDelete, "/companies/"+created.ID, nil, app.token)
		if status != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", status, body)
		}

		status, _ = app.do(t, http.MethodGet, "/companies/"+created.ID, nil, "")
		if status != http.StatusNotFound {
			t.Fatalf("expected 404 after delete, got %d", status)
		}
	})
}

func setupTestApp(t *testing.T) testApp {
	t.Helper()
	if os.Getenv("INTEGRATION_TESTS") != "1" {
		t.Skip("set INTEGRATION_TESTS=1 to run integration tests")
	}
	if os.Getenv("DOCKER_API_VERSION") == "" {
		t.Setenv("DOCKER_API_VERSION", "1.44")
	}

	ctx := context.Background()
	container, err := tcpostgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		tcpostgres.WithDatabase("company_service"),
		tcpostgres.WithUsername("company"),
		tcpostgres.WithPassword("company"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{
		URL:             databaseURL,
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	migration, err := os.ReadFile("../../migrations/000001_create_companies.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	logger := zerolog.New(io.Discard)
	service := appcompany.NewService(postgres.NewCompanyRepository(pool), noopProducer{}, &logger)
	jwtManager := auth.NewJWTManager(testJWTSecret)
	router := httpapi.NewRouter(service, jwtManager, pool, logger)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	token, err := jwtManager.Generate("integration-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	return testApp{server: server, token: token}
}

func (a testApp) do(t *testing.T, method string, path string, payload any, token string) (int, []byte) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, a.server.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return res.StatusCode, raw
}

func mustDecode(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, body)
	}
}
