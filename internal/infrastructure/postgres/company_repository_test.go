//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"company-service/internal/config"
	companydomain "company-service/internal/domain/company"
	"company-service/internal/infrastructure/postgres"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCompanyRepositoryIntegration(t *testing.T) {
	ctx := context.Background()
	repo := setupCompanyRepository(t, ctx)

	t.Run("create and get company", func(t *testing.T) {
		created := testCompany("Acme")

		if err := repo.Create(ctx, created); err != nil {
			t.Fatal(err)
		}

		byID, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		assertCompanyEqual(t, created, byID)

		byName, err := repo.GetByName(ctx, created.Name)
		if err != nil {
			t.Fatal(err)
		}
		assertCompanyEqual(t, created, byName)
	})

	t.Run("create maps duplicate name", func(t *testing.T) {
		created := testCompany("DupCo")
		if err := repo.Create(ctx, created); err != nil {
			t.Fatal(err)
		}

		duplicateName := testCompany(created.Name)
		duplicateName.ID = uuid.New()

		err := repo.Create(ctx, duplicateName)
		if !errors.Is(err, companydomain.ErrDuplicateName) {
			t.Fatalf("expected ErrDuplicateName, got %v", err)
		}
	})

	t.Run("create maps duplicate id", func(t *testing.T) {
		created := testCompany("DupID")
		if err := repo.Create(ctx, created); err != nil {
			t.Fatal(err)
		}

		duplicateID := testCompany("OtherDupID")
		duplicateID.ID = created.ID

		err := repo.Create(ctx, duplicateID)
		if !errors.Is(err, companydomain.ErrDuplicateID) {
			t.Fatalf("expected ErrDuplicateID, got %v", err)
		}
	})

	t.Run("update persists fields", func(t *testing.T) {
		created := testCompany("PatchCo")
		if err := repo.Create(ctx, created); err != nil {
			t.Fatal(err)
		}

		created.Name = "Patched"
		created.Description = "updated description"
		created.AmountOfEmployees = 42
		created.Registered = false
		created.Type = companydomain.TypeNonProfit
		created.UpdatedAt = created.UpdatedAt.Add(time.Hour)

		if err := repo.Update(ctx, created); err != nil {
			t.Fatal(err)
		}

		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		assertCompanyEqual(t, created, got)
	})

	t.Run("update maps missing company", func(t *testing.T) {
		missing := testCompany("MissingUpd")

		err := repo.Update(ctx, missing)
		if !errors.Is(err, companydomain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("delete removes company", func(t *testing.T) {
		created := testCompany("DeleteCo")
		if err := repo.Create(ctx, created); err != nil {
			t.Fatal(err)
		}

		if err := repo.Delete(ctx, created.ID); err != nil {
			t.Fatal(err)
		}

		_, err := repo.GetByID(ctx, created.ID)
		if !errors.Is(err, companydomain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("delete maps missing company", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		if !errors.Is(err, companydomain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("get maps missing company", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		if !errors.Is(err, companydomain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound by id, got %v", err)
		}

		_, err = repo.GetByName(ctx, "NoSuchCompany")
		if !errors.Is(err, companydomain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound by name, got %v", err)
		}
	})
}

func setupCompanyRepository(t *testing.T, ctx context.Context) *postgres.CompanyRepository {
	t.Helper()
	if os.Getenv("INTEGRATION_TESTS") != "1" {
		t.Skip("set INTEGRATION_TESTS=1 to run integration tests")
	}
	if os.Getenv("DOCKER_API_VERSION") == "" {
		t.Setenv("DOCKER_API_VERSION", "1.44")
	}

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

	migration, err := os.ReadFile("../../../migrations/000001_create_companies.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	return postgres.NewCompanyRepository(pool)
}

func testCompany(name string) *companydomain.Company {
	createdAt := time.Date(2026, time.May, 12, 7, 0, 0, 123456000, time.UTC)

	return &companydomain.Company{
		ID:                uuid.New(),
		Name:              name,
		Description:       "description",
		AmountOfEmployees: 10,
		Registered:        true,
		Type:              companydomain.TypeCorporations,
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt.Add(time.Minute),
	}
}

func assertCompanyEqual(t *testing.T, want *companydomain.Company, got *companydomain.Company) {
	t.Helper()

	if got == nil {
		t.Fatal("expected company, got nil")
	}
	if got.ID != want.ID {
		t.Fatalf("expected id %s, got %s", want.ID, got.ID)
	}
	if got.Name != want.Name {
		t.Fatalf("expected name %q, got %q", want.Name, got.Name)
	}
	if got.Description != want.Description {
		t.Fatalf("expected description %q, got %q", want.Description, got.Description)
	}
	if got.AmountOfEmployees != want.AmountOfEmployees {
		t.Fatalf("expected employees %d, got %d", want.AmountOfEmployees, got.AmountOfEmployees)
	}
	if got.Registered != want.Registered {
		t.Fatalf("expected registered %t, got %t", want.Registered, got.Registered)
	}
	if got.Type != want.Type {
		t.Fatalf("expected type %q, got %q", want.Type, got.Type)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("expected created_at %s, got %s", want.CreatedAt, got.CreatedAt)
	}
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("expected updated_at %s, got %s", want.UpdatedAt, got.UpdatedAt)
	}
}
