//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	appcompany "company-service/internal/application/company"
	"company-service/internal/config"
	"company-service/internal/infrastructure/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestOutboxRepositoryIntegration(t *testing.T) {
	ctx := context.Background()
	repo, pool := setupOutboxRepository(t, ctx)

	t.Run("store and fetch pending events", func(t *testing.T) {
		truncateOutboxEvents(t, ctx, pool)
		first := testOutboxEvent(appcompany.EventCompanyCreated, time.Now().UTC().Add(-2*time.Minute))
		second := testOutboxEvent(appcompany.EventCompanyUpdated, time.Now().UTC().Add(-time.Minute))

		if err := repo.Store(ctx, second); err != nil {
			t.Fatal(err)
		}
		if err := repo.Store(ctx, first); err != nil {
			t.Fatal(err)
		}

		events, err := repo.FetchPending(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("expected one event, got %d", len(events))
		}
		assertOutboxEventEqual(t, first, events[0])
		if events[0].Attempts != 0 {
			t.Fatalf("expected attempts 0, got %d", events[0].Attempts)
		}
	})

	t.Run("fetch skips unavailable and published events", func(t *testing.T) {
		truncateOutboxEvents(t, ctx, pool)
		future := testOutboxEvent(appcompany.EventCompanyCreated, time.Now().UTC().Add(time.Hour))
		published := testOutboxEvent(appcompany.EventCompanyUpdated, time.Now().UTC().Add(-time.Minute))

		if err := repo.Store(ctx, future); err != nil {
			t.Fatal(err)
		}
		if err := repo.Store(ctx, published); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkPublished(ctx, published.ID); err != nil {
			t.Fatal(err)
		}

		events, err := repo.FetchPending(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Fatalf("expected no pending events, got %d", len(events))
		}
	})

	t.Run("mark published stores timestamp and clears last error", func(t *testing.T) {
		truncateOutboxEvents(t, ctx, pool)
		event := testOutboxEvent(appcompany.EventCompanyDeleted, time.Now().UTC().Add(-time.Minute))

		if err := repo.Store(ctx, event); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE outbox_events SET last_error = $1 WHERE id = $2", "previous error", event.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkPublished(ctx, event.ID); err != nil {
			t.Fatal(err)
		}

		var publishedAt sql.NullTime
		var lastError sql.NullString
		if err := pool.QueryRow(ctx, "SELECT published_at, last_error FROM outbox_events WHERE id = $1", event.ID).
			Scan(&publishedAt, &lastError); err != nil {
			t.Fatal(err)
		}
		if !publishedAt.Valid {
			t.Fatal("expected published_at to be set")
		}
		if lastError.Valid {
			t.Fatalf("expected last_error to be cleared, got %q", lastError.String)
		}
	})

	t.Run("mark failed increments attempts and delays availability", func(t *testing.T) {
		truncateOutboxEvents(t, ctx, pool)
		event := testOutboxEvent(appcompany.EventCompanyCreated, time.Now().UTC().Add(-time.Minute))
		publishErr := errors.New("publish failed")

		if err := repo.Store(ctx, event); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkFailed(ctx, event.ID, publishErr); err != nil {
			t.Fatal(err)
		}

		var attempts int
		var lastError string
		var availableAt time.Time
		if err := pool.QueryRow(ctx, "SELECT attempts, last_error, available_at FROM outbox_events WHERE id = $1", event.ID).
			Scan(&attempts, &lastError, &availableAt); err != nil {
			t.Fatal(err)
		}
		if attempts != 1 {
			t.Fatalf("expected attempts 1, got %d", attempts)
		}
		if lastError != publishErr.Error() {
			t.Fatalf("expected last_error %q, got %q", publishErr.Error(), lastError)
		}
		if !availableAt.After(time.Now().UTC()) {
			t.Fatalf("expected available_at to be delayed, got %s", availableAt)
		}

		events, err := repo.FetchPending(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Fatalf("expected failed event to be delayed, got %d pending events", len(events))
		}
	})
}

func setupOutboxRepository(t *testing.T, ctx context.Context) (*postgres.OutboxRepository, *pgxpool.Pool) {
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

	migration, err := os.ReadFile("../../../migrations/000002_create_outbox_events.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	return postgres.NewOutboxRepository(pool), pool
}

func truncateOutboxEvents(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "TRUNCATE outbox_events"); err != nil {
		t.Fatal(err)
	}
}

func testOutboxEvent(eventType string, occurredAt time.Time) appcompany.Event {
	companyID := uuid.New()

	return appcompany.Event{
		ID:         uuid.New(),
		Type:       eventType,
		OccurredAt: occurredAt,
		CompanyID:  companyID,
	}
}

func assertOutboxEventEqual(t *testing.T, want appcompany.Event, got appcompany.OutboxEvent) {
	t.Helper()

	if got.ID != want.ID {
		t.Fatalf("expected outbox id %s, got %s", want.ID, got.ID)
	}
	if got.Event.ID != want.ID {
		t.Fatalf("expected event id %s, got %s", want.ID, got.Event.ID)
	}
	if got.Event.Type != want.Type {
		t.Fatalf("expected event type %q, got %q", want.Type, got.Event.Type)
	}
	if got.Event.CompanyID != want.CompanyID {
		t.Fatalf("expected company id %s, got %s", want.CompanyID, got.Event.CompanyID)
	}
}
